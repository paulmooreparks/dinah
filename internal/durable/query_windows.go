//go:build windows

package durable

import "golang.org/x/sys/windows"

// The three opens below read no file's content. Each answers a raw handle
// that shares read, write and delete, so it never refuses another process's
// rename or removal, and its caller closes it with windows.CloseHandle. They
// live here because the guard in guard_test.go refuses windows.CreateFile to
// every other package.

// OpenQuery opens a file or a directory for FILE_READ_ATTRIBUTES alone, the
// access a query of its attributes, its final path or its volume needs.
// FILE_FLAG_BACKUP_SEMANTICS is what lets CreateFile open a directory: its
// documentation names it as the flag "to obtain a handle to a directory".
func OpenQuery(path string) (windows.Handle, error) {
	return openHandle(path, windows.FILE_READ_ATTRIBUTES, windows.FILE_FLAG_BACKUP_SEMANTICS)
}

// OpenReparsePoint is OpenQuery on the reparse point a path names rather than
// on what it points at. It rests on CreateFile's FILE_FLAG_OPEN_REPARSE_POINT:
// "Normal reparse point processing will not occur; CreateFile will attempt to
// open the reparse point." and "If the file is not a reparse point, then this
// flag is ignored."
func OpenReparsePoint(path string) (windows.Handle, error) {
	return openHandle(path, windows.FILE_READ_ATTRIBUTES, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT)
}

// OpenWatch opens a directory for ReadDirectoryChangesW, whose documentation
// says "This directory must be opened with the FILE_LIST_DIRECTORY access
// right". FILE_FLAG_OVERLAPPED makes each call on the handle asynchronous.
func OpenWatch(path string) (windows.Handle, error) {
	return openHandle(path, windows.FILE_LIST_DIRECTORY, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OVERLAPPED)
}

// openHandle makes one CreateFile call on an existing path with every sharing
// flag and answers the handle.
func openHandle(path string, access, flags uint32) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return windows.InvalidHandle, err
	}
	return windows.CreateFile(name, access, shareAll, nil, windows.OPEN_EXISTING, flags, 0)
}
