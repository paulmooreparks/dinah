//go:build linux || darwin

package agents

import (
	"errors"
	"os"
	"syscall"
)

func snapshotOpenedIdentity(_ *os.File, info os.FileInfo) (os.FileInfo, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, errors.New("file identity metadata is unavailable")
	}
	return &identityFileInfo{info, FileIdentity{uint64(stat.Dev), 0, uint64(stat.Ino)}}, nil
}
