//go:build windows

package agents

import (
	"golang.org/x/sys/windows"
	"os"
)

func snapshotOpenedIdentity(file *os.File, info os.FileInfo) (os.FileInfo, error) {
	raw, err := file.SyscallConn()
	if err != nil {
		return nil, err
	}
	var data windows.ByHandleFileInformation
	var queryErr error
	if err := raw.Control(func(handle uintptr) { queryErr = windows.GetFileInformationByHandle(windows.Handle(handle), &data) }); err != nil {
		return nil, err
	}
	if queryErr != nil {
		return nil, queryErr
	}
	return &identityFileInfo{info, FileIdentity{uint64(data.VolumeSerialNumber), uint64(data.FileIndexHigh), uint64(data.FileIndexLow)}}, nil
}
