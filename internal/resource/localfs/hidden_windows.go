//go:build windows

package localfs

import (
	"os"

	"golang.org/x/sys/windows"
)

// PathHidden reports whether the entry at path has the Windows hidden or
// system attribute.
func PathHidden(path string) (bool, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}
	attributes, err := windows.GetFileAttributes(p)
	return attributes&(windows.FILE_ATTRIBUTE_HIDDEN|windows.FILE_ATTRIBUTE_SYSTEM) != 0, err
}

// FileHidden reports whether an open file has the Windows hidden or system
// attribute.
func FileHidden(f *os.File) (bool, error) {
	var information windows.ByHandleFileInformation
	err := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &information)
	return information.FileAttributes&(windows.FILE_ATTRIBUTE_HIDDEN|windows.FILE_ATTRIBUTE_SYSTEM) != 0, err
}
