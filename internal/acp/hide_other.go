//go:build !windows

package acp

import (
	"os/exec"
	"syscall"
	"time"
)

func hideWindow(cmd *exec.Cmd) {}

// tieToApp does nothing outside Windows; the agent's process ends with the
// app's pipes there.
func tieToApp(cmd *exec.Cmd) {}

// endWhole starts the command in a process group of its own and, when its
// context is cancelled, sends the group SIGKILL, which kill(2) delivers to
// every process in the group when given the group's ID negated. WaitDelay
// then bounds how long Wait waits for output still held open after that.
func endWhole(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 5 * time.Second
}
