//go:build !windows

package main

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

// startDetached starts the new build in a session of its own, so it
// outlives the window.
func startDetached(path string, args ...string) error {
	cmd := exec.Command(path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// waitForProcess waits for a process to end by asking, twice a second,
// whether it still exists. It reports whether the process has ended.
func waitForProcess(pid int, limit time.Duration) bool {
	end := time.Now().Add(limit)
	for time.Now().Before(end) {
		p, err := os.FindProcess(pid)
		if err != nil || p.Signal(syscall.Signal(0)) != nil {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}
