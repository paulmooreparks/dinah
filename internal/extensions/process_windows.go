//go:build windows

package extensions

import (
	"errors"
	"golang.org/x/sys/windows"
	"os/exec"
	"syscall"
)

func hideProcess(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
func controlProcessAlive(pid int) (bool, error) {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(h)
	status, err := windows.WaitForSingleObject(h, 0)
	return status == uint32(windows.WAIT_TIMEOUT), err
}
