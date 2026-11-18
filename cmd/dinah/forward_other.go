//go:build !windows

package main

import "os/exec"

// withoutNewConsole does nothing outside Windows, where a program started
// by another opens no window of its own.
func withoutNewConsole(cmd *exec.Cmd) {}
