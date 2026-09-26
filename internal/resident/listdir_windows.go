//go:build windows

package resident

import (
	"encoding/binary"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// listDir stats and lists one directory through a single handle that shares
// read, write and delete, and closes it the moment the enumeration ends,
// before any file in the directory is read. os.ReadDir opens a directory
// without FILE_SHARE_DELETE (syscall.Open's share mode is FILE_SHARE_READ and
// FILE_SHARE_WRITE), so a directory the resident was listing refused another
// process's delete of it for the length of the listing; this is the directory
// form of openShared. The one handle also answers the directory's own stat,
// where os.Stat opened a second one.
//
// The enumeration is GetFileInformationByHandleEx with the
// FileFullDirectoryRestartInfo class and then FileFullDirectoryInfo, whose
// documentation describes the first as "Identical to FileFullDirectoryInfo,
// but forces the enumeration operation to start again from the beginning",
// until the call answers ERROR_NO_MORE_FILES. Each record is a
// FILE_FULL_DIR_INFO, read at the offsets that structure's documented layout
// gives: NextEntryOffset at 0, FileAttributes at 56, FileNameLength (in
// bytes) at 60, and FileName, UTF-16, at 68. The first call may instead
// answer ERROR_FILE_NOT_FOUND, which on a handle just opened on an existing
// directory can only mean that nothing matched, so it ends the listing the
// same way.
//
// The entries are sorted by name, as os.ReadDir sorts them. An entry's type
// is a directory or a regular file from its attributes; a reparse point takes
// its type from os.Lstat, as os.ReadDir reports the link rather than its
// target, and one that has vanished since the record was read is dropped.
func listDir(path string) (fs.FileInfo, []fs.DirEntry, error) {
	file, err := openDirShared(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	handle := windows.Handle(file.Fd())
	info, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() {
		return info, nil, nil
	}
	var entries []fs.DirEntry
	// FILE_FULL_DIR_INFO's documentation requires the structure to be
	// aligned on a LONGLONG (8-byte) boundary, and a []byte carries no such
	// promise (on the stack it has none), so the buffer is made of uint64s.
	words := make([]uint64, 8<<10)
	buf := unsafe.Slice((*byte)(unsafe.Pointer(&words[0])), 8*len(words))
	class := uint32(windows.FileFullDirectoryRestartInfo)
	for {
		err := windows.GetFileInformationByHandleEx(handle, class, &buf[0], uint32(len(buf)))
		first := class == windows.FileFullDirectoryRestartInfo
		class = windows.FileFullDirectoryInfo
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) || first && errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
			break
		}
		if err != nil {
			return nil, nil, &os.PathError{Op: "readdir", Path: path, Err: err}
		}
		for offset := 0; ; {
			record := buf[offset:]
			if len(record) < fullDirInfoName {
				return nil, nil, &os.PathError{Op: "readdir", Path: path, Err: errors.New("a directory record runs past the buffer")}
			}
			length := int(binary.LittleEndian.Uint32(record[60:]))
			if fullDirInfoName+length > len(record) {
				return nil, nil, &os.PathError{Op: "readdir", Path: path, Err: errors.New("a directory record's name runs past the buffer")}
			}
			units := make([]uint16, length/2)
			for i := range units {
				units[i] = binary.LittleEndian.Uint16(record[fullDirInfoName+2*i:])
			}
			entryName := string(utf16.Decode(units))
			if entryName != "." && entryName != ".." {
				if entry, ok := listedEntryOf(path, entryName, binary.LittleEndian.Uint32(record[56:])); ok {
					entries = append(entries, entry)
				}
			}
			next := int(binary.LittleEndian.Uint32(record[0:]))
			if next == 0 {
				break
			}
			offset += next
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	if listDirHeld != nil {
		listDirHeld(path)
	}
	return info, entries, nil
}

// openDirShared opens a directory, or a file, for reading with all three
// share modes. CreateFile opens a directory only when it is given
// FILE_FLAG_BACKUP_SEMANTICS, which its documentation names as the flag "to
// obtain a handle to a directory".
func openDirShared(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(handle), path), nil
}

// fullDirInfoName is the offset of FILE_FULL_DIR_INFO's FileName member.
const fullDirInfoName = 68

// listedEntryOf answers the entry for one record, or false when a reparse
// point has vanished since it was listed.
func listedEntryOf(dir, name string, attributes uint32) (fs.DirEntry, bool) {
	entry := &listedEntry{dir: dir, name: name}
	switch {
	case attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0:
		info, err := os.Lstat(filepath.Join(dir, name))
		if err != nil {
			return nil, false
		}
		entry.typ = info.Mode().Type()
	case attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0:
		entry.typ = fs.ModeDir
	}
	return entry, true
}
