//go:build windows

package agents

import (
	"encoding/binary"
	"errors"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// fileIDInfo follows the documented FILE_ID_INFO layout.
type fileIDInfo struct {
	VolumeSerialNumber uint64
	FileID             [16]byte
}

func queryFileID(handle windows.Handle, data *fileIDInfo) error {
	return windows.GetFileInformationByHandleEx(handle, windows.FileIdInfo, (*byte)(unsafe.Pointer(data)), uint32(unsafe.Sizeof(*data)))
}

func snapshotOpenedIdentity(file *os.File, info os.FileInfo) (os.FileInfo, error) {
	return snapshotOpenedIdentityWithQuery(file, info, queryFileID)
}

func snapshotOpenedIdentityWithQuery(file *os.File, info os.FileInfo, query func(windows.Handle, *fileIDInfo) error) (os.FileInfo, error) {
	raw, err := file.SyscallConn()
	if err != nil {
		return nil, err
	}
	var data fileIDInfo
	var queryErr error
	if err := raw.Control(func(handle uintptr) { queryErr = query(windows.Handle(handle), &data) }); err != nil {
		return nil, err
	}
	if queryErr != nil {
		if errors.Is(queryErr, windows.ERROR_INVALID_PARAMETER) || errors.Is(queryErr, windows.ERROR_NOT_SUPPORTED) || errors.Is(queryErr, windows.ERROR_INVALID_FUNCTION) {
			return &unavailableIdentityFileInfo{info, UnsupportedFileIdentityReason}, nil
		}
		return nil, queryErr
	}
	return &identityFileInfo{info, FileIdentity{data.VolumeSerialNumber, binary.LittleEndian.Uint64(data.FileID[8:]), binary.LittleEndian.Uint64(data.FileID[:8])}}, nil
}
