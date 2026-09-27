//go:build windows

package resident

import (
	"io/fs"
	"path/filepath"
	"unsafe"

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
// An entry whose attributes lack FILE_ATTRIBUTE_REPARSE_POINT is not a link
// and costs nothing more: the attributes are the FileAttributes member of the
// FILE_FULL_DIR_INFO record listDir read for it, so this first test opens
// nothing. For an entry that carries the attribute, the answer is linkAt's.
func isLink(dir string, entry fs.DirEntry) bool {
	if listed, ok := entry.(*listedEntry); ok && listed.attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
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
// The open rests on CreateFile's FILE_FLAG_OPEN_REPARSE_POINT: "Normal reparse
// point processing will not occur; CreateFile will attempt to open the reparse
// point. When a file is opened, a file handle is returned, whether or not the
// filter that controls the reparse point is operational." and "If the file is
// not a reparse point, then this flag is ignored." The query rests on
// FILE_INFO_BY_HANDLE_CLASS's FileAttributeTagInfo: "File attribute
// information should be retrieved. Used for any handles. Use only when calling
// GetFileInformationByHandleEx. See FILE_ATTRIBUTE_TAG_INFO." The test of the
// tag is IsReparseTagNameSurrogate's: "Determines whether a tag's associated
// reparse point is a surrogate for another named entity (for example, a
// mounted folder)." and "A nonzero return value means that the tag indicates
// a surrogate reparse point." link.go cites the bit that macro tests.
func linkAt(path string) bool {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return true
	}
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
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
