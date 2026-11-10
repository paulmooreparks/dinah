//go:build windows

package savelock

import (
	"os"

	"golang.org/x/sys/windows"
)

// openLockFile opens the lock file, making it if it is not there, and shares
// it for reading and writing only. Without FILE_SHARE_DELETE the file cannot
// be removed or renamed while this handle is open, which CreateFile
// documents, so a holder's lock file stays the one at its path for as long
// as the lock is held.
// https://learn.microsoft.com/windows/win32/api/fileapi/nf-fileapi-createfilew
func openLockFile(path string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil,
		windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(h), path), nil
}
