//go:build windows

package procjob

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Restart to Update starts the new build with CREATE_BREAKAWAY_FROM_JOB from
// a window that is in the app's job. The new build must start outside the
// job, or it ends when the old window closes.
func TestChildCanBreakAwayFromTheJob(t *testing.T) {
	if err := TieRequired(os.Getpid()); err != nil {
		t.Fatalf("joining the job: %v", err)
	}
	inJob := func(pid int) bool {
		t.Helper()
		p, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
		if err != nil {
			t.Fatalf("opening process %d: %v", pid, err)
		}
		defer windows.CloseHandle(p)
		var member int32
		probe := windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessInJob")
		if ok, _, err := probe.Call(uintptr(p), uintptr(appJob()), uintptr(unsafe.Pointer(&member))); ok == 0 {
			t.Fatalf("IsProcessInJob: %v", err)
		}
		return member != 0
	}
	start := func(flags uint32) *exec.Cmd {
		t.Helper()
		cmd := exec.Command("cmd.exe", "/c", "ping", "-n", "5", "127.0.0.1")
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: flags | windows.CREATE_NO_WINDOW, HideWindow: true}
		if err := cmd.Start(); err != nil {
			t.Fatalf("starting child with flags %#x: %v", flags, err)
		}
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
		return cmd
	}

	inside := start(0)
	if !inJob(inside.Process.Pid) {
		t.Fatal("a child started normally should be in the job")
	}
	outside := start(windows.CREATE_BREAKAWAY_FROM_JOB)
	if inJob(outside.Process.Pid) {
		t.Fatal("a child started with CREATE_BREAKAWAY_FROM_JOB should be outside the job")
	}
}
