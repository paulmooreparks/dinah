//go:build windows

package resident

import (
	"io/fs"
	"path/filepath"
	"unsafe"

	"dinah/internal/durable"
	"golang.org/x/sys/windows"
)

// fileAttributeTagInfo is FILE_ATTRIBUTE_TAG_INFO, declared here because
// golang.org/x/sys v0.47.0 declares the information class FileAttributeTagInfo
// and not the struct. Its page says of ReparseTag only "The reparse tag."; the
// condition under which it carries one is [MS-FSCC]'s, for
// FileAttributeTagInformation: "If the FileAttributes member includes the
// FILE_ATTRIBUTE_REPARSE_POINT attribute flag, this member specifies the
// reparse tag. Otherwise, this member SHOULD be set to 0, and MUST be
// ignored."
type fileAttributeTagInfo struct {
	FileAttributes uint32
	ReparseTag     uint32
}

// isLink judges an entry a build, a re-list or a reconcile is about to read.
// An entry whose type carries neither fs.ModeSymlink nor fs.ModeIrregular is
// not a link and costs nothing more, because the type is what listDir's
// (*os.File).ReadDir answered and this first test opens nothing. For any
// other entry the answer is linkAt's, which reads the reparse tag itself.
//
// The first test rests on two documented statements. fs.DirEntry's Type
// "returns the type bits for the entry. The type bits are a subset of the
// usual FileMode bits, those returned by the FileMode.Type method." The Go
// 1.23 release notes say of those bits on Windows: "Mount points no longer
// have ModeSymlink set, and reparse points that are not symlinks, Unix
// sockets, or dedup files now always have ModeIrregular set." So a symbolic
// link carries ModeSymlink and every other reparse point but a socket or a
// deduplicated file carries ModeIrregular. [MS-FSCC] section 2.1.2.1 gives
// IO_REPARSE_TAG_AF_UNIX as 0x80000023 and IO_REPARSE_TAG_DEDUP as
// 0x80000013, and neither sets the name-surrogate bit link.go tests, so no
// name surrogate passes the first test.
//
// What the first test cannot see: the same notes say "This behavior is
// controlled by the winsymlink setting", and a process started with
// GODEBUG=winsymlink=0 gets the mapping Go used before 1.23, which the notes
// do not describe. Under it a name surrogate other than a symbolic link or a
// mount point may read as a regular file or a directory, and the resident
// would then hold what it points at.
func isLink(dir string, entry fs.DirEntry) bool {
	if entry.Type()&(fs.ModeSymlink|fs.ModeIrregular) == 0 {
		return false
	}
	return linkAt(filepath.Join(dir, entry.Name()))
}

// isLinkPath is isLink for a path a reconcile names, whose attributes no
// listing has read: GetFileAttributes answers them, and a path that answers
// none is left to the reconcile's own stat.
func isLinkPath(path string) bool {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attributes, err := windows.GetFileAttributes(name)
	if err != nil || attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
		return false
	}
	return linkAt(path)
}

// linkAt opens a reparse point itself, reads its tag, and answers whether the
// tag's name-surrogate bit is set. A failed open or query answers true, so an
// entry the resident cannot judge is read from disk rather than held, which
// errs toward the answer that is always current. The handle lasts only for
// the query, inside the pass that reads the entry.
//
// The open is durable.OpenReparsePoint, which rests on CreateFile's
// FILE_FLAG_OPEN_REPARSE_POINT: "Normal reparse point processing will not
// occur; CreateFile will attempt to open the reparse point. When a file is
// opened, a file handle is returned, whether or not the filter that controls
// the reparse point is operational." and "If the file is not a reparse point,
// then this flag is ignored." The query rests on
// FILE_INFO_BY_HANDLE_CLASS's FileAttributeTagInfo: "File attribute
// information should be retrieved. Used for any handles. Use only when calling
// GetFileInformationByHandleEx. See FILE_ATTRIBUTE_TAG_INFO." The test of the
// tag is IsReparseTagNameSurrogate's: "Determines whether a tag's associated
// reparse point is a surrogate for another named entity (for example, a
// mounted folder)." and "A nonzero return value means that the tag indicates
// a surrogate reparse point." link.go cites the bit that macro tests.
func linkAt(path string) bool {
	handle, err := durable.OpenReparsePoint(path)
	if err != nil {
		return true
	}
	var info fileAttributeTagInfo
	err = windows.GetFileInformationByHandleEx(handle, windows.FileAttributeTagInfo, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	windows.CloseHandle(handle)
	if err != nil {
		return true
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
		return false
	}
	return nameSurrogate(info.ReparseTag)
}
