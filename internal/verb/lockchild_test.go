package verb

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
)

// lockHelperVariable, when set in the environment of this test binary, makes
// TestLockHelperProcess hold a lock instead of passing. It is the twin of the
// helper in internal/bench's tests, which this package cannot import. Every
// test that starts a child starts it from this binary, keeps its
// *os.Process, and ends it through that handle alone.
const (
	lockHelperVariable = "DINAH_VERB_LOCK_HELPER"
	lockHelperActor    = "DINAH_VERB_LOCK_HELPER_ACTOR"
)

// TestLockHelperProcess is the helper's whole life when lockHelperVariable
// names a directory: it acquires that directory's lock through bench.Acquire,
// prints "held", and waits on its standard input until its parent ends it.
// Run as an ordinary test it asserts nothing and returns at once.
func TestLockHelperProcess(t *testing.T) {
	dir := os.Getenv(lockHelperVariable)
	if dir == "" {
		return
	}
	if _, err := bench.Acquire(dir, os.Getenv(lockHelperActor), "2026-09-28T00:00:00Z"); err != nil {
		fmt.Println("refused", err)
		os.Exit(1)
	}
	fmt.Println("held")
	io.Copy(io.Discard, os.Stdin)
	os.Exit(0)
}

// lockChild is one helper process this test started.
type lockChild struct {
	cmd   *exec.Cmd
	ended bool
}

// holdInChild starts the helper holding dir's lock as actor, waits for it to
// say it holds it, and registers its ending with the test.
func holdInChild(t *testing.T, dir, actor string) *lockChild {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("find the test binary: %v", err)
	}
	cmd := exec.Command(self, "-test.run=^TestLockHelperProcess$")
	cmd.Env = append(os.Environ(), lockHelperVariable+"="+dir, lockHelperActor+"="+actor)
	if _, err := cmd.StdinPipe(); err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start the helper: %v", err)
	}
	child := &lockChild{cmd: cmd}
	t.Cleanup(child.end)
	said, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(said) != "held" {
		t.Fatalf("the helper said %q (%v), wanted held", said, err)
	}
	return child
}

// end kills the helper through its own handle and waits for it, which leaves
// its lock the way a process that died leaves one.
func (c *lockChild) end() {
	if c.ended {
		return
	}
	c.ended = true
	c.cmd.Process.Kill()
	c.cmd.Wait()
}

// endAndWaitDead ends the helper and waits, for at most ten seconds, for the
// lock it held to be judged dead. The Windows LockFileEx documentation says
// the operating system unlocks a terminated process's locks after a time that
// "depends upon available system resources", so the first judgement can still
// find the lock held.
func (c *lockChild) endAndWaitDead(t *testing.T, lockPath string) {
	t.Helper()
	c.end()
	deadline := time.Now().Add(10 * time.Second)
	for {
		_, verdict := bench.JudgeLock(lockPath)
		if verdict == bench.VerdictDead {
			return
		}
		if verdict != bench.VerdictLive || time.Now().After(deadline) {
			t.Fatalf("the lock at %s judged %s after its holder ended, wanted dead", lockPath, verdict)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
