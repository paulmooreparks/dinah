//go:build windows

package savelock

import (
	"encoding/binary"

	"golang.org/x/sys/windows"
)

// folderIdentity is the volume serial number and file index of a folder,
// which Windows gives for an open handle (GetFileInformationByHandle). The
// folder is opened without the flag that stops at a link, so a junction or a
// symbolic link gives the identity of the folder it leads to.
//
// One call is made and its failure refuses the save. An earlier build asked
// for the longer FILE_ID_INFO first and fell back to this one, which needed
// a reading of error codes to tell "not offered" from "failed", and two
// writers that read them differently would have chosen different locks.
//
// The documentation allows this identity to be coarser than a folder: a
// file system with longer identifiers may give two folders one index, and a
// network server may return only part of the information. Either makes two
// folders share a lock, so their saves wait for each other, and neither
// makes one folder have two.
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
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		return nil, err
	}
	out := make([]byte, 0, 24)
	out = append(out, "index\x00"...)
	out = binary.LittleEndian.AppendUint32(out, info.VolumeSerialNumber)
	out = binary.LittleEndian.AppendUint32(out, info.FileIndexHigh)
	return binary.LittleEndian.AppendUint32(out, info.FileIndexLow), nil
}
