package savelock

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Two ways of writing one folder's path give one lock, and so do two
// spellings of a name that differ in case. Another folder or another name
// gives another.
func TestOneTargetHasOneLockHoweverItIsWritten(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	same := key(dir, "notes.md")
	for name, k := range map[string]string{
		"through a subfolder and back": key(filepath.Join(sub, ".."), "notes.md"),
		"with a trailing separator":    key(dir+string(filepath.Separator), "notes.md"),
		"the name in capitals":         key(dir, "NOTES.md"),
	} {
		if k != same {
			t.Errorf("%s gives another lock", name)
		}
	}
	if link := filepath.Join(dir, "link"); os.Symlink(sub, link) == nil {
		if key(link, "notes.md") != key(sub, "notes.md") {
			t.Error("a link to the folder gives another lock")
		}
	}
	if key(sub, "notes.md") == same || key(dir, "other.md") == same {
		t.Error("another folder or another name gives the same lock")
	}
	if id, err := folderIdentity(dir); err != nil || len(id) == 0 {
		t.Errorf("the folder has no identity here: %v", err)
	}
}

// The lock excludes a second holder until it is released, and a writer that
// will not wait is told the file is busy.
func TestASecondHolderWaitsOrIsToldTheFileIsBusy(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	release, err := Hold(ctx, dir, "notes.md", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Hold(ctx, dir, "notes.md", 100*time.Millisecond); !errors.Is(err, ErrBusy) {
		t.Fatalf("a second holder: %v", err)
	}
	other, err := Hold(ctx, dir, "other.md", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("another file's lock: %v", err)
	}
	other()
	done := make(chan error, 1)
	go func() {
		again, err := Hold(ctx, dir, "notes.md", 5*time.Second)
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
	held, err := Hold(ctx, dir, "notes.md", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer held()
	cancel()
	if _, err := Hold(cancelled, dir, "notes.md", 5*time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("a holder whose context ended: %v", err)
	}
}

// The lock holds between processes, which is what it is for: another process
// that has it keeps this one out, and its ending lets this one in.
func TestTheLockHoldsBetweenProcesses(t *testing.T) {
	if dir := os.Getenv("SAVELOCK_TEST_HOLD"); dir != "" {
		release, err := Hold(context.Background(), dir, "notes.md", time.Second)
		if err != nil {
			os.Stdout.WriteString("failed " + err.Error() + "\n")
			return
		}
		defer release()
		os.Stdout.WriteString("held\n")
		bufio.NewReader(os.Stdin).ReadString('\n')
		return
	}
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestTheLockHoldsBetweenProcesses$")
	cmd.Env = append(os.Environ(), "SAVELOCK_TEST_HOLD="+dir)
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
	if _, err := Hold(ctx, dir, "notes.md", 200*time.Millisecond); !errors.Is(err, ErrBusy) {
		t.Fatalf("this process, while the other holds the lock: %v", err)
	}
	stdin.Write([]byte("\n"))
	cmd.Wait()
	release, err := Hold(ctx, dir, "notes.md", 5*time.Second)
	if err != nil {
		t.Fatalf("this process, after the other ended: %v", err)
	}
	release()
}
