//go:build windows

package acp

import (
	"os/exec"
	"syscall"
	"time"

	"studio/internal/procjob"
)

// hideWindow keeps the agent's console window from appearing when the desktop
// app, which has no console of its own, starts it.
func hideWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= 0x08000000 // CREATE_NO_WINDOW
}

// endWhole bounds how long Wait waits, after the command's context is
// cancelled and the command ended, for output that a process it started still
// holds open.
func endWhole(cmd *exec.Cmd) {
	cmd.WaitDelay = 5 * time.Second
}

// tieToApp puts a started process in the app's job object, so no agent
// outlives the app and holds its sessions open (procjob).
func tieToApp(cmd *exec.Cmd) {
	if cmd.Process != nil {
		procjob.Tie(cmd.Process.Pid)
	}
}
