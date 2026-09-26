//go:build windows

package resident

import (
	"os"

	"golang.org/x/sys/windows"
)

// openShared opens a file for reading with all three share modes. os.Open
// shares read and write but not delete, and a file the resident is reading
// then refuses another process's delete of it, or a rename of it, for the
// length of the read; the CreateFile documentation says of FILE_SHARE_DELETE
// that it "Enables subsequent open operations on a file or device to request
// delete access", and "Delete access allows both delete and rename
// operations". The watch handle shares delete for the same reason.
func openShared(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(handle), path), nil
}
