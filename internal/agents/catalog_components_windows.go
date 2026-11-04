package agents

import (
	"errors"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// fileAttributeTagInfo follows FILE_ATTRIBUTE_TAG_INFO's two DWORD fields.
// https://learn.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-file_attribute_tag_info
type fileAttributeTagInfo struct {
	FileAttributes uint32
	ReparseTag     uint32
}

// The documented N bit implements IsReparseTagNameSurrogate.
// https://learn.microsoft.com/en-us/windows/win32/fileio/reparse-point-tags
func catalogNameSurrogate(attributes, tag uint32) bool {
	return attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 && tag&0x20000000 != 0
}

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
		var info fileAttributeTagInfo
		err = windows.GetFileInformationByHandleEx(h, windows.FileAttributeTagInfo, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
		windows.CloseHandle(h)
		if err != nil {
			return err
		}
		if catalogNameSurrogate(info.FileAttributes, info.ReparseTag) {
			return errors.New("catalogue entries must not redirect through links")
		}
	}
	return nil
}
