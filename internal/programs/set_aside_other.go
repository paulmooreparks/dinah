//go:build !windows

package programs

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

const asideRemovalSupported = false

func inspectAside(path string) (os.FileInfo, string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, "", err
	}
	return info, fmt.Sprintf("%v", info.Sys().(*syscall.Stat_t)), nil
}

func runningAsideImages() (map[string][]uint32, []string, error) {
	return map[string][]uint32{}, []string{"Process inspection and removal are supported only on Windows."}, nil
}

func removeAside(file asideFile, check func() error) (string, error) {
	return "unsupported", errors.New("removing set-aside programs is supported only on Windows")
}
