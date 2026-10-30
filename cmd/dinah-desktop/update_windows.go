//go:build windows

package main

import (
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// startDetached starts the new build outside any job object the window is
// in, so it outlives the window. CREATE_BREAKAWAY_FROM_JOB is refused when
// the window's own job does not allow breaking away
// (https://learn.microsoft.com/windows/win32/procthread/process-creation-flags);
// the program is then started without it, and the log says so, because a
// job that ends its processes when the window closes will end it too.
// Dinah's own job allows breaking away; a job the window was started in
// by another program may not.
func startDetached(path string, args ...string) error {
	cmd := exec.Command(path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_BREAKAWAY_FROM_JOB | windows.CREATE_NEW_PROCESS_GROUP}
	err := cmd.Start()
	if err == nil {
		return cmd.Process.Release()
	}
	logf("core", "warn", "starting the new build of %s outside the window's job failed (%v); starting it inside", product.Name, err)
	cmd = exec.Command(path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// waitForProcess waits for a process to end, through its handle, which is
// a documented waitable object; a process that has already ended cannot be
// opened, and there is nothing to wait for. It reports whether the process
// has ended.
func waitForProcess(pid int, limit time.Duration) bool {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return true
	}
	defer windows.CloseHandle(h)
	ev, err := windows.WaitForSingleObject(h, uint32(limit/time.Millisecond))
	return err == nil && ev == windows.WAIT_OBJECT_0
}
