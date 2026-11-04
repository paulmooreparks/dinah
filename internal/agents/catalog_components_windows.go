package agents

import (
	"errors"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// Inspect the object itself rather than inferring redirection from its name.
// CreateFile documents OPEN_REPARSE_POINT as opening the reparse point itself.
// https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew
func checkCatalogComponents(path string) error {
	volume := filepath.VolumeName(path)
	current := volume + string(filepath.Separator)
	for _, part := range strings.Split(strings.TrimPrefix(path, current), string(filepath.Separator)) {
		current = filepath.Join(current, part)
		p, err := windows.UTF16PtrFromString(current)
		if err != nil {
			return err
		}
		h, err := windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
		if err != nil {
			return err
		}
		var info windows.ByHandleFileInformation
		err = windows.GetFileInformationByHandle(h, &info)
		windows.CloseHandle(h)
		if err != nil {
			return err
		}
		if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return errors.New("catalogue entries must not redirect through reparse points")
		}
	}
	return nil
}
