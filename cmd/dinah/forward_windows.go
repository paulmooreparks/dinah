//go:build windows

package main

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

var procGetConsoleWindow = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleWindow")

// withoutNewConsole keeps a console program the command program starts from
// opening a console window of its own. An agent program starts the command
// program with pipes and no console, as it starts an MCP server. Windows
// documents that a console program inherits its parent's console, and that
// the system otherwise creates a new console, with a window, when it starts
// a console program; the person would see that window. GetConsoleWindow returns NULL
// when the calling process has no console, and the child is then started
// with CREATE_NO_WINDOW; its input and output are the pipes it is given.
// When the command program has a console, the child shares it, as Windows
// does by default.
// https://learn.microsoft.com/windows/console/getconsolewindow
// https://learn.microsoft.com/windows/console/creation-of-a-console
// https://learn.microsoft.com/windows/win32/procthread/process-creation-flags
func withoutNewConsole(cmd *exec.Cmd) {
	if window, _, _ := procGetConsoleWindow.Call(); window != 0 {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW
}
