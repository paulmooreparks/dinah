package statefile

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"studio/internal/filelock"
)

func put(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestInterruptedReplacementRecovery(t *testing.T) {
	for _, phase := range []string{"before replacement", "missing target", "after replacement"} {
		t.Run(phase, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			old, next := []byte(`{"value":"old"}`), []byte(`{"value":"new"}`)
			if phase == "before replacement" {
				put(t, path, string(old))
			}
			if phase == "after replacement" {
				put(t, path, string(next))
			}
			tx, _ := json.Marshal(transaction{Version: 1, Previous: old, Next: next})
			put(t, path+".pending", string(tx))
			got, err := Read(path)
			if err != nil || string(got) != string(next) {
				t.Fatalf("recovery failed: %s %v", got, err)
			}
			if _, err := os.Stat(path + ".pending"); !os.IsNotExist(err) {
				t.Fatal("committed transaction not removed")
			}
		})
	}
	path := filepath.Join(t.TempDir(), "state.json")
	put(t, path, `{"value":"external"}`)
	tx, _ := json.Marshal(transaction{Version: 1, Previous: []byte(`{"value":"old"}`), Next: []byte(`{"value":"new"}`)})
	put(t, path+".pending", string(tx))
	if _, err := Read(path); err == nil {
		t.Fatal("external edit overwritten by recovery")
	}
	got, _ := os.ReadFile(path)
	if string(got) != `{"value":"external"}` {
		t.Fatal("external edit lost")
	}
}

func TestUpdateReadsChangesAndSkips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "list.json")
	if _, err := ReadShared(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a missing file reads as %v", err)
	}
	if err := Update(path, func(previous []byte) ([]byte, error) {
		if previous != nil {
			t.Fatalf("a missing file is given as %q", previous)
		}
		return []byte(`{"n":1}`), nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := Update(path, func(previous []byte) ([]byte, error) {
		if string(previous) != `{"n":1}` {
			t.Fatalf("previous = %q", previous)
		}
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadShared(path); err != nil || string(got) != `{"n":1}` {
		t.Fatalf("a skipped update changed the file: %s %v", got, err)
	}
	refused := errors.New("refused")
	if err := Update(path, func([]byte) ([]byte, error) { return []byte(`{"n":2}`), refused }); !errors.Is(err, refused) {
		t.Fatalf("the change's error is %v", err)
	}
	if err := Update(path, func([]byte) ([]byte, error) { return []byte(`not json`), nil }); err == nil {
		t.Fatal("invalid JSON written")
	}
	if got, _ := ReadShared(path); string(got) != `{"n":1}` {
		t.Fatalf("a refused update changed the file: %s", got)
	}
	// The lock is released: another open of the lock file takes it at once.
	f, err := os.OpenFile(path+".lock", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if locked, err := filelock.TryLock(f); err != nil || !locked {
		t.Fatalf("the lock outlived the update: %v", err)
	}
	filelock.Unlock(f)
}

// The helper process holds a file's lock until its input closes, or counts
// into the file, as the command line would while the app does the same.
func TestLockHelper(t *testing.T) {
	path, mode := os.Getenv("DINAH_STATEFILE_HELPER"), os.Getenv("DINAH_STATEFILE_MODE")
	if path == "" {
		t.Skip("run by the tests across processes")
	}
	switch mode {
	case "hold":
		_ = Update(path, func(previous []byte) ([]byte, error) {
			os.Stdout.WriteString("held\n")
			_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
			return nil, nil
		})
	case "count":
		for range 25 {
			if err := Update(path, increment); err != nil {
				os.Stdout.WriteString("error " + err.Error() + "\n")
				return
			}
		}
		os.Stdout.WriteString("counted\n")
	case "alternate":
		os.Stdout.WriteString("ready\n")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		for range 25 {
			if err := Update(path, appendLetter("b")); err != nil {
				os.Stdout.WriteString("error " + err.Error() + "\n")
				return
			}
		}
		os.Stdout.WriteString("counted\n")
	}
}

// appendLetter changes a file's content by adding a letter to the record of
// who changed it, in order.
func appendLetter(letter string) func([]byte) ([]byte, error) {
	return func(previous []byte) ([]byte, error) {
		var v struct{ S string }
		if previous != nil {
			if err := json.Unmarshal(previous, &v); err != nil {
				return nil, err
			}
		}
		v.S += letter
		return json.Marshal(v)
	}
}

func helper(t *testing.T, path, mode string) (*exec.Cmd, io.WriteCloser, *bufio.Reader) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockHelper$")
	cmd.Env = append(os.Environ(), "DINAH_STATEFILE_HELPER="+path, "DINAH_STATEFILE_MODE="+mode)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return cmd, stdin, bufio.NewReader(stdout)
}

func increment(previous []byte) ([]byte, error) {
	var v struct{ N int }
	if previous != nil {
		if err := json.Unmarshal(previous, &v); err != nil {
			return nil, err
		}
	}
	v.N++
	return json.Marshal(v)
}

// Another program's lock is waited for, and released by the system when
// that program ends without releasing it.
func TestUpdateWaitsForAnotherProgramsLock(t *testing.T) {
	wait := LockWait
	t.Cleanup(func() { LockWait = wait })
	LockWait = 300 * time.Millisecond
	path := filepath.Join(t.TempDir(), "list.json")
	cmd, stdin, out := helper(t, path, "hold")
	if line, _ := out.ReadString('\n'); line != "held\n" {
		t.Fatalf("the helper did not take the lock: %q", line)
	}
	if err := Update(path, func([]byte) ([]byte, error) { return []byte(`{}`), nil }); err == nil {
		t.Fatal("another program's lock was ignored")
	}
	if _, err := ReadShared(path); err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a read did not wait for another program's lock: %v", err)
	}
	// Other state files are not held up by the wait.
	if err := Write(filepath.Join(filepath.Dir(path), "other.json"), []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	stdin.Close()
	if err := Update(path, func([]byte) ([]byte, error) { return []byte(`{}`), nil }); err != nil {
		t.Fatalf("the lock of an ended program was not released: %v", err)
	}
}

// Two programs updating one file at once lose none of each other's changes.
func TestUpdatesAcrossProcessesKeepEveryChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "count.json")
	cmd, stdin, out := helper(t, path, "count")
	defer stdin.Close()
	for range 25 {
		if err := Update(path, increment); err != nil {
			t.Fatal(err)
		}
	}
	if line, _ := out.ReadString('\n'); line != "counted\n" {
		t.Fatalf("the helper failed: %q", line)
	}
	_ = cmd.Wait()
	got, err := ReadShared(path)
	if err != nil || string(got) != `{"N":50}` {
		t.Fatalf("got %s %v", got, err)
	}
}

// Updates from many writers at once lose none of their changes.
func TestConcurrentUpdatesKeepEveryChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "count.json")
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if err := Update(path, increment); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	got, err := ReadShared(path)
	if err != nil || string(got) != `{"N":20}` {
		t.Fatalf("got %s %v", got, err)
	}
}
