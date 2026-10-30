package statefile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"studio/internal/filelock"
)

// Writers of one program wait their turn in a queue that has no deadline of
// its own: LockWait is spent only waiting for another program. Ten writers
// that each hold the lock for 100 ms take a second in all, three times the
// wait, and none of them gives up.
func TestWritersOfOneProgramQueueWithoutADeadline(t *testing.T) {
	wait := LockWait
	t.Cleanup(func() { LockWait = wait })
	LockWait = 300 * time.Millisecond
	path := filepath.Join(t.TempDir(), "queue.json")
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			err := Update(path, func(previous []byte) ([]byte, error) {
				time.Sleep(100 * time.Millisecond)
				return increment(previous)
			})
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	got, err := ReadShared(path)
	if err != nil || string(got) != `{"N":10}` {
		t.Fatalf("got %s %v", got, err)
	}
}

// A writer waiting for another program's data lock holds the gate, and gives
// it up when it has the data lock, not when it has written. A program that
// has just released the data lock therefore finds the gate taken.
func TestTheGateIsHeldWhileWaitingForTheDataLockAndReleasedOnceItIsWon(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gate.json")
	data, err := os.OpenFile(path+".lock", os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer data.Close()
	if locked, err := filelock.TryLock(data); err != nil || !locked {
		t.Fatalf("the test could not take the data lock: %v", err)
	}
	type outcome struct {
		unlock func()
		err    error
	}
	won := make(chan outcome, 1)
	go func() {
		unlock, err := lock(path)
		won <- outcome{unlock, err}
	}()
	gate, err := os.OpenFile(path+".gate", os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Close()
	// The waiter takes the gate within a moment and keeps it.
	taken := false
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		locked, err := filelock.TryLock(gate)
		if err != nil {
			t.Fatal(err)
		}
		if !locked {
			taken = true
			break
		}
		filelock.Unlock(gate)
	}
	if !taken {
		t.Fatal("the waiting writer did not take the gate")
	}
	time.Sleep(100 * time.Millisecond)
	if locked, _ := filelock.TryLock(gate); locked {
		t.Fatal("the gate was given up while the writer still waited for the data lock")
	}
	if err := filelock.Unlock(data); err != nil {
		t.Fatal(err)
	}
	var got outcome
	select {
	case got = <-won:
	case <-time.After(3 * time.Second):
		t.Fatal("the waiting writer never got the data lock")
	}
	if got.err != nil {
		t.Fatal(got.err)
	}
	defer got.unlock()
	// The writer holds the data lock and has released the gate.
	if locked, err := filelock.TryLock(gate); err != nil || !locked {
		t.Fatalf("the gate was still held after the data lock was won: %v", err)
	}
	filelock.Unlock(gate)
}

// Two programs that update one file in turn alternate: a program that has
// just changed it does not change it again ahead of the other, which is
// waiting. Without the gate the one that released the lock asks again before
// the other's next try, and its whole run of 25 comes first.
func TestTwoProgramsRunsAlternate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "turns.json")
	cmd, stdin, out := helper(t, path, "alternate")
	defer stdin.Close()
	if line, _ := out.ReadString('\n'); line != "ready\n" {
		t.Fatalf("the helper was not ready: %q", line)
	}
	if _, err := stdin.Write([]byte("go\n")); err != nil {
		t.Fatal(err)
	}
	for range 25 {
		if err := Update(path, appendLetter("a")); err != nil {
			t.Fatal(err)
		}
	}
	if line, _ := out.ReadString('\n'); line != "counted\n" {
		t.Fatalf("the helper failed: %q", line)
	}
	_ = cmd.Wait()
	got, err := ReadShared(path)
	if err != nil {
		t.Fatal(err)
	}
	var v struct{ S string }
	if json.Unmarshal(got, &v) != nil || len(v.S) != 50 {
		t.Fatalf("got %s", got)
	}
	switches := 0
	for i := 1; i < len(v.S); i++ {
		if v.S[i] != v.S[i-1] {
			switches++
		}
	}
	// Starting together, the two take turns for as long as both have more to
	// do. The run of one program alone at the start and at the end is not a
	// failure to take turns.
	if switches < 20 {
		t.Fatalf("the programs changed the file in runs, not in turns (%d changes of program in 50): %s", switches, v.S)
	}
}
