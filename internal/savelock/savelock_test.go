package savelock

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// own gives the test a lock file of its own, so it neither waits on nor
// delays the tests of another package.
func own(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	t.Setenv(TestDirEnv, d)
	return d
}

// The lock excludes a second holder until it is released, and a writer that
// will not wait is told another program is saving.
func TestASecondHolderWaitsOrIsToldAnotherProgramIsSaving(t *testing.T) {
	own(t)
	ctx := context.Background()
	release, err := Hold(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Hold(ctx, 100*time.Millisecond); !errors.Is(err, ErrBusy) {
		t.Fatalf("a second holder: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		again, err := Hold(ctx, 5*time.Second)
		if err == nil {
			again()
		}
		done <- err
	}()
	time.Sleep(150 * time.Millisecond)
	release()
	if err := <-done; err != nil {
		t.Fatalf("the waiting holder: %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	held, err := Hold(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer held()
	cancel()
	if _, err := Hold(cancelled, 5*time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("a holder whose context ended: %v", err)
	}
}

// There is one lock file, at a fixed name, whatever is being saved.
func TestThereIsOneLockFile(t *testing.T) {
	d := own(t)
	release, err := Hold(context.Background(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	release()
	entries, err := os.ReadDir(d)
	if err != nil || len(entries) != 1 || entries[0].Name() != file {
		t.Fatalf("the lock folder holds %v %v", entries, err)
	}
}

// On Windows the held lock file cannot be removed, so no second writer can
// be given a new file of the same name while the first still works.
func TestTheHeldLockFileCannotBeRemovedOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("elsewhere a file can be removed while it is open")
	}
	path := filepath.Join(own(t), file)
	release, err := Hold(context.Background(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err == nil {
		t.Fatal("the lock file was removed while it was held")
	}
	if err := os.Rename(path, path+".moved"); err == nil {
		t.Fatal("the lock file was renamed while it was held")
	}
	release()
	if err := os.Remove(path); err != nil {
		t.Fatalf("the lock file could not be removed after its release: %v", err)
	}
}

// The lock holds between processes, which is what it is for: another process
// that has it keeps this one out, and its ending lets this one in.
func TestTheLockHoldsBetweenProcesses(t *testing.T) {
	if os.Getenv("SAVELOCK_TEST_HOLD") != "" {
		release, err := Hold(context.Background(), time.Second)
		if err != nil {
			os.Stdout.WriteString("failed " + err.Error() + "\n")
			return
		}
		defer release()
		os.Stdout.WriteString("held\n")
		bufio.NewReader(os.Stdin).ReadString('\n')
		return
	}
	own(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestTheLockHoldsBetweenProcesses$")
	cmd.Env = append(os.Environ(), "SAVELOCK_TEST_HOLD=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "held" {
		t.Fatalf("the other process said %q %v", line, err)
	}
	ctx := context.Background()
	if _, err := Hold(ctx, 200*time.Millisecond); !errors.Is(err, ErrBusy) {
		t.Fatalf("this process, while the other holds the lock: %v", err)
	}
	stdin.Write([]byte("\n"))
	cmd.Wait()
	release, err := Hold(ctx, 5*time.Second)
	if err != nil {
		t.Fatalf("this process, after the other ended: %v", err)
	}
	release()
}
