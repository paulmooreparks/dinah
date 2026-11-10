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

func keyOf(t *testing.T, folder string) string {
	t.Helper()
	k, err := key(folder)
	if err != nil {
		t.Fatalf("%s: %v", folder, err)
	}
	return k
}

// Two ways of writing one folder's path give one lock, and another folder
// gives another.
func TestOneFolderHasOneLockHoweverItsPathIsWritten(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	same := keyOf(t, dir)
	for name, folder := range map[string]string{
		"through a subfolder and back": filepath.Join(sub, ".."),
		"with a trailing separator":    dir + string(filepath.Separator),
	} {
		if keyOf(t, folder) != same {
			t.Errorf("%s gives another lock", name)
		}
	}
	if link := filepath.Join(dir, "link"); os.Symlink(sub, link) == nil {
		if keyOf(t, link) != keyOf(t, sub) {
			t.Error("a link to the folder gives another lock")
		}
	}
	if keyOf(t, sub) == same {
		t.Error("another folder gives the same lock")
	}
	if id, err := folderIdentity(dir); err != nil || len(id) == 0 {
		t.Errorf("the folder has no identity here: %v", err)
	}
}

// A folder whose identity cannot be read is not saved to: a guessed lock
// could differ from another writer's. A file system that says it has no
// identity to give is the one exception, and there the resolved path is the
// same for every writer.
func TestAnUnreadableIdentityRefusesTheSave(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() { identityOf = folderIdentity })
	identityOf = func(string) ([]byte, error) { return nil, errors.New("the device is not ready") }
	if _, err := Hold(context.Background(), dir, time.Second); !errors.Is(err, ErrNoIdentity) {
		t.Fatalf("a failed identity: %v", err)
	}
	identityOf = func(string) ([]byte, error) { return nil, nil }
	if _, err := key(dir); !errors.Is(err, ErrNoIdentity) {
		t.Fatalf("an empty identity: %v", err)
	}
	identityOf = func(string) ([]byte, error) { return nil, errUnsupported }
	first, err := key(dir)
	if err != nil {
		t.Fatalf("a file system with no identity to give: %v", err)
	}
	if second, _ := key(filepath.Join(dir, ".")); second != first {
		t.Error("the path that stands in for the identity is not the same for the same folder")
	}
	if _, err := key(filepath.Join(dir, "no such folder")); !errors.Is(err, ErrNoIdentity) {
		t.Fatalf("a folder that is not there: %v", err)
	}
}

// The lock excludes a second holder until it is released, for any file of
// the folder, and a writer that will not wait is told the folder is busy.
func TestASecondHolderWaitsOrIsToldTheFolderIsBusy(t *testing.T) {
	ctx := context.Background()
	dir, other := t.TempDir(), t.TempDir()
	release, err := Hold(ctx, dir, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Hold(ctx, dir, 100*time.Millisecond); !errors.Is(err, ErrBusy) {
		t.Fatalf("a second holder: %v", err)
	}
	elsewhere, err := Hold(ctx, other, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("another folder's lock: %v", err)
	}
	elsewhere()
	done := make(chan error, 1)
	go func() {
		again, err := Hold(ctx, dir, 5*time.Second)
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
	held, err := Hold(ctx, dir, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer held()
	cancel()
	if _, err := Hold(cancelled, dir, 5*time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("a holder whose context ended: %v", err)
	}
}

// On Windows a held lock file cannot be removed, so no second writer can be
// given a new file of the same name while the first still works.
func TestAHeldLockFileCannotBeRemovedOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("elsewhere a file can be removed while it is open")
	}
	dir := t.TempDir()
	release, err := Hold(context.Background(), dir, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	locks, _ := dirOfLocks(t)
	path := filepath.Join(locks, keyOf(t, dir))
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

func dirOfLocks(t *testing.T) (string, error) {
	t.Helper()
	d, err := dir()
	if err != nil {
		t.Fatal(err)
	}
	return d, nil
}

// The lock holds between processes, which is what it is for: another process
// that has it keeps this one out, and its ending lets this one in.
func TestTheLockHoldsBetweenProcesses(t *testing.T) {
	if dir := os.Getenv("SAVELOCK_TEST_HOLD"); dir != "" {
		release, err := Hold(context.Background(), dir, time.Second)
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
	if _, err := Hold(ctx, dir, 200*time.Millisecond); !errors.Is(err, ErrBusy) {
		t.Fatalf("this process, while the other holds the lock: %v", err)
	}
	stdin.Write([]byte("\n"))
	cmd.Wait()
	release, err := Hold(ctx, dir, 5*time.Second)
	if err != nil {
		t.Fatalf("this process, after the other ended: %v", err)
	}
	release()
}
