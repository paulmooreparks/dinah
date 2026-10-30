package acp

import "os/exec"

// HideWindow keeps a child process's console window from appearing, as the
// agent's is kept. Call it before the process starts.
func HideWindow(cmd *exec.Cmd) { hideWindow(cmd) }

// TieToApp puts a started child process in the app's job, as the agent is
// put, so that it ends when the app does.
func TieToApp(cmd *exec.Cmd) { tieToApp(cmd) }

// EndWhole makes cancelling a command's context end the processes it started
// as well as the command, so that a shell's child, such as a long sleep or a
// test run, does not hold the command's output open and keep it from
// finishing. Call it before the process starts.
func EndWhole(cmd *exec.Cmd) { endWhole(cmd) }
