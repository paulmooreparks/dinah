//go:build windows

package savelock

import (
	"encoding/binary"
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

// fileIDInfo follows the documented FILE_ID_INFO layout.
type fileIDInfo struct {
	VolumeSerialNumber uint64
	FileID             [16]byte
}

// folderIdentity is the volume and file identifier of a folder, which
// Windows gives for an open handle. The folder is opened without the flag
// that stops at a link, so a junction or a symbolic link gives the identity
// of the folder it leads to. FILE_ID_INFO is asked for first, and
// BY_HANDLE_FILE_INFORMATION where the file system says it does not give
// that. A file system that gives neither is reported as errUnsupported, and
// any other failure as itself, which refuses the save.
// https://learn.microsoft.com/windows/win32/api/winbase/nf-winbase-getfileinformationbyhandleex
// https://learn.microsoft.com/windows/win32/api/fileapi/nf-fileapi-getfileinformationbyhandle
func folderIdentity(folder string) ([]byte, error) {
	p, err := windows.UTF16PtrFromString(folder)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(h)
	var id fileIDInfo
	err = windows.GetFileInformationByHandleEx(h, windows.FileIdInfo, (*byte)(unsafe.Pointer(&id)), uint32(unsafe.Sizeof(id)))
	if err == nil {
		out := make([]byte, 0, 32)
		out = append(out, "id\x00"...)
		out = binary.LittleEndian.AppendUint64(out, id.VolumeSerialNumber)
		return append(out, id.FileID[:]...), nil
	}
	if !unsupported(err) {
		return nil, err
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		if unsupported(err) {
			return nil, errUnsupported
		}
		return nil, err
	}
	out := make([]byte, 0, 24)
	out = append(out, "index\x00"...)
	out = binary.LittleEndian.AppendUint32(out, info.VolumeSerialNumber)
	out = binary.LittleEndian.AppendUint32(out, info.FileIndexHigh)
	return binary.LittleEndian.AppendUint32(out, info.FileIndexLow), nil
}

// unsupported says an error is one of those Windows gives for a kind of
// information a file system does not implement.
func unsupported(err error) bool {
	return errors.Is(err, windows.ERROR_INVALID_PARAMETER) || errors.Is(err, windows.ERROR_NOT_SUPPORTED) || errors.Is(err, windows.ERROR_INVALID_FUNCTION)
}
