//go:build !windows

package extensions

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func hideProcess(cmd *exec.Cmd) {}
func controlProcessAlive(pid int) (bool, error) {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false, err
	}
	defer p.Release()
	err = p.Signal(syscall.Signal(0))
	if errors.Is(err, syscall.ESRCH) || errors.Is(err, os.ErrProcessDone) {
		return false, nil
	}
	if errors.Is(err, syscall.EPERM) {
		return true, nil
	}
	return err == nil, err
}
