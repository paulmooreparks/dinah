//go:build windows

package terminals

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

type startupObservation struct {
	output           []byte
	err, contextErr  error
	killErr          error
	deadlineObserved bool
	duration         time.Duration
	capture          string
}

// No context or WaitDelay owns this child. The timer selects observation,
// then capture completes before this function requests termination and reaps.
func observeStartupCommand(command *exec.Cmd, bound time.Duration, capture func(uint32) string) (result startupObservation) {
	started := time.Now()
	timer := time.NewTimer(bound)
	defer timer.Stop()
	defer func() { result.duration = time.Since(started) }()
	output, err := os.CreateTemp("", "node-version-output-*.log")
	if err != nil {
		result.err = err
		return
	}
	defer os.Remove(output.Name())
	defer output.Close()
	command.Stdout, command.Stderr = output, output
	if err := command.Start(); err != nil {
		result.err = err
		return
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	reaped := false
	defer func() {
		if !reaped {
			_ = command.Process.Kill()
			<-done
		}
	}()
	awaitStartupExit(command, done, timer.C, capture, &result)
	reaped = true
	result.output, err = os.ReadFile(output.Name())
	if err != nil {
		result.err = fmt.Errorf("read query output: %w (process error %v)", err, result.err)
	}
	return
}

func awaitStartupExit(command *exec.Cmd, done <-chan error, deadline <-chan time.Time, capture func(uint32) string, result *startupObservation) {
	select {
	case result.err = <-done:
	case <-deadline:
		// Prefer an already observed ordinary exit over the timer. An exit
		// during capture remains a deadline observation and is reported.
		select {
		case result.err = <-done:
		default:
			result.deadlineObserved = true
			result.contextErr = context.DeadlineExceeded
			if capture != nil {
				result.capture = capture(uint32(command.Process.Pid))
			}
			result.killErr = command.Process.Kill()
			result.err = <-done
		}
	}
}

func TestStartupCapturePrecedesTerminationAndReaps(t *testing.T) {
	node, err := exec.LookPath("node.exe")
	if err != nil {
		t.Skip("node.exe is not installed")
	}
	dir := t.TempDir()
	t.Setenv("DINAH_TEST_NATIVE_SCRATCH_DIR", dir)
	started := time.Now()
	command := exec.Command(node, "-e", "setInterval(()=>{},1000)")
	var process windows.Handle
	aliveBefore, aliveAfter := false, false
	result := observeStartupCommand(command, 100*time.Millisecond, func(pid uint32) string {
		process, err = windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
		if err != nil {
			return err.Error()
		}
		state, waitErr := windows.WaitForSingleObject(process, 0)
		aliveBefore = waitErr == nil && state == uint32(windows.WAIT_TIMEOUT)
		diagnostic := nodeDumpDiagnostic(os.Getpid(), started, pid)
		state, waitErr = windows.WaitForSingleObject(process, 0)
		aliveAfter = waitErr == nil && state == uint32(windows.WAIT_TIMEOUT)
		return diagnostic
	})
	if process != 0 {
		defer windows.CloseHandle(process)
	}
	if !result.deadlineObserved || result.contextErr != context.DeadlineExceeded || result.err == nil || result.killErr != nil || !aliveBefore || !aliveAfter {
		t.Fatalf("capture/termination ordering failed: %+v alive=%t/%t", result, aliveBefore, aliveAfter)
	}
	if command.ProcessState == nil || !command.ProcessState.Exited() {
		t.Fatal("child was not reaped")
	}
	state, err := windows.WaitForSingleObject(process, 0)
	if err != nil || state != uint32(windows.WAIT_OBJECT_0) {
		t.Fatalf("child remains alive after return: %d %v", state, err)
	}
	files, err := filepath.Glob(filepath.Join(dir, fmt.Sprintf("node-startup-%d-*.dmp", command.Process.Pid)))
	if err != nil || len(files) != 1 {
		t.Fatalf("capture did not retain dump: %v %v %s", files, err, result.capture)
	}
	assertDumpHasThreads(t, files[0])

	testStartupOrdinaryExitOrdering(t)
}

func TestStartupOrdinaryExitFixture(t *testing.T) {
	if os.Getenv("DINAH_STARTUP_ORDINARY_FIXTURE") == "1" {
		fmt.Println("ordinary-exit")
	}
}

func testStartupOrdinaryExitOrdering(t *testing.T) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// Startup has its own bound and result. Only after Wait has completed do
	// we offer exit and deadline together to the sequencing decision.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestStartupOrdinaryExitFixture$")
	command.Env = append(os.Environ(), "DINAH_STARTUP_ORDINARY_FIXTURE=1")
	command.WaitDelay = 500 * time.Millisecond
	started := time.Now()
	output, err := command.CombinedOutput()
	t.Logf("ordinary fixture startup/exit: duration=%s error=%v context=%v", time.Since(started), err, ctx.Err())
	if err != nil || ctx.Err() != nil || !strings.Contains(string(output), "ordinary-exit") {
		t.Fatalf("ordinary fixture did not complete before sequencing check: error=%v context=%v output=%q", err, ctx.Err(), output)
	}
	for i := 0; i < 100; i++ {
		done := make(chan error, 1)
		done <- nil
		deadline := make(chan time.Time, 1)
		deadline <- time.Now()
		var ordinary startupObservation
		awaitStartupExit(command, done, deadline, func(uint32) string {
			t.Error("completed exit triggered capture")
			return ""
		}, &ordinary)
		if ordinary.err != nil || ordinary.deadlineObserved || ordinary.contextErr != nil || ordinary.killErr != nil {
			t.Fatalf("completed ordinary exit was reclassified: %+v", ordinary)
		}
	}
}
