package bench

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// lockHelperVariable, when set in the environment of this test binary, makes
// TestMain run the lock helper instead of the tests. Its value is the mode.
// Every test that starts a child starts it from this binary, keeps its
// *os.Process, and ends it through that handle alone.
const lockHelperVariable = "DINAH_LOCK_HELPER"

// The helper's other inputs, read from its environment.
const (
	lockHelperDirs  = "DINAH_LOCK_HELPER_DIRS"
	lockHelperActor = "DINAH_LOCK_HELPER_ACTOR"
	// lockHelperEnded carries the parent's endedProcesses as JSON.
	lockHelperEnded = "DINAH_LOCK_HELPER_ENDED"
)

// The helper's modes.
const (
	// helperHold acquires the lock of every directory named, prints "held"
	// and its own start, and then waits to be ended.
	helperHold = "hold"
	// helperIdle takes no lock, prints "idle" and its own start, and waits.
	helperIdle = "idle"
	// helperReclaim acquires the lock of the one directory named, pausing at
	// ReclaimInterpose to print "interposed" and read a line, then prints
	// "held" or "refused" and the error, and waits.
	helperReclaim = "reclaim"
)

// runLockHelper is the child's whole life. It answers an exit code only when
// it fails before it can wait; otherwise it waits on its standard input
// until its parent ends it.
func runLockHelper(mode string) int {
	readEndedList()
	actor := os.Getenv(lockHelperActor)
	dirs := filepath.SplitList(os.Getenv(lockHelperDirs))
	_, start := selfIdentity()
	stdin := bufio.NewReader(os.Stdin)
	switch mode {
	case helperHold:
		for _, dir := range dirs {
			if _, err := Acquire(dir, actor, "2026-09-28T00:00:00Z"); err != nil {
				fmt.Println("refused", err)
				return 1
			}
		}
		fmt.Println("held", start)
	case helperIdle:
		fmt.Println("idle", start)
	case helperReclaim:
		ReclaimInterpose = func() {
			fmt.Println("interposed")
			stdin.ReadString('\n')
		}
		if _, err := Acquire(dirs[0], actor, "2026-09-28T00:00:00Z"); err != nil {
			fmt.Println("refused", err)
		} else {
			fmt.Println("held", start)
		}
	default:
		return 2
	}
	io.Copy(io.Discard, stdin)
	return 0
}

// lockChild is one helper process this test started.
type lockChild struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	out   *bufio.Reader
	ended bool
	// start is the start the helper reported with its first "held" or
	// "idle", which is what endedProcesses records it under.
	start string
}

// startLockChild starts the helper in a mode over some directories, acting as
// actor, and registers its ending with the test.
func startLockChild(t *testing.T, mode, actor string, dirs ...string) *lockChild {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("find the test binary: %v", err)
	}
	cmd := exec.Command(self, "-test.run=^$")
	cmd.Env = append(os.Environ(),
		lockHelperVariable+"="+mode,
		lockHelperActor+"="+actor,
		lockHelperDirs+"="+strings.Join(dirs, string(os.PathListSeparator)),
		lockHelperEnded+"="+endedList(),
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start the helper: %v", err)
	}
	child := &lockChild{cmd: cmd, stdin: stdin, out: bufio.NewReader(stdout)}
	t.Cleanup(func() { child.end(t) })
	return child
}

// line reads the helper's next line of output, failing the test when the
// helper ended without one.
func (c *lockChild) line(t *testing.T) string {
	t.Helper()
	text, err := c.out.ReadString('\n')
	if err != nil {
		t.Fatalf("read the helper's output: %v (read %q)", err, text)
	}
	return strings.TrimSpace(text)
}

// expect reads the helper's next line and fails unless it begins with word,
// answering the rest of the line.
func (c *lockChild) expect(t *testing.T, word string) string {
	t.Helper()
	text := c.line(t)
	rest, ok := strings.CutPrefix(text, word)
	if !ok {
		t.Fatalf("the helper said %q, wanted a line beginning %q", text, word)
	}
	rest = strings.TrimSpace(rest)
	if (word == "held" || word == "idle") && c.start == "" {
		c.start = rest
	}
	return rest
}

// proceed lets a helper paused at ReclaimInterpose carry on.
func (c *lockChild) proceed(t *testing.T) {
	t.Helper()
	if _, err := io.WriteString(c.stdin, "go\n"); err != nil {
		t.Fatalf("tell the helper to proceed: %v", err)
	}
}

// pid answers the helper's process identifier.
func (c *lockChild) pid() int {
	return c.cmd.Process.Pid
}

// end kills the helper through its own handle and waits for it, which is what
// a process that died without releasing anything looks like.
func (c *lockChild) end(t *testing.T) {
	if c.ended {
		return
	}
	c.ended = true
	c.cmd.Process.Kill()
	c.cmd.Wait()
	if c.start != "" {
		endedProcesses.Lock()
		endedProcesses.starts[c.pid()] = c.start
		endedProcesses.Unlock()
	}
}

// endedProcesses are the helpers this test binary has ended and waited on,
// each PID with the start its helper reported. A helper learns its parent's
// list through lockHelperEnded.
var endedProcesses = struct {
	sync.Mutex
	starts map[int]string
}{starts: map[int]string{}}

// endedByThisTestOrGone is the rule 6 every test here judges by: a record
// naming a helper this binary ended, by PID and by the start that helper
// reported, is gone, and every other record goes to the platform's own rule.
// Windows gives a freed PID to the next process quickly, and a process this
// one cannot open answers not gone, so without this the verdict on a helper
// the test ended would depend on what else the machine started since. Rule 6
// itself is tested apart from it, against a table the test controls and
// against a process the test holds alive.
func endedByThisTestOrGone(record LockRecord) bool {
	endedProcesses.Lock()
	start, ended := endedProcesses.starts[record.PID]
	endedProcesses.Unlock()
	if ended && start == record.Start {
		return true
	}
	return recordedProcessGone(record)
}

// endedList renders endedProcesses for lockHelperEnded.
func endedList() string {
	endedProcesses.Lock()
	defer endedProcesses.Unlock()
	encoded, _ := json.Marshal(endedProcesses.starts)
	return string(encoded)
}

// readEndedList fills endedProcesses from lockHelperEnded in a helper.
func readEndedList() {
	var starts map[int]string
	if json.Unmarshal([]byte(os.Getenv(lockHelperEnded)), &starts) != nil {
		return
	}
	endedProcesses.Lock()
	defer endedProcesses.Unlock()
	for pid, start := range starts {
		endedProcesses.starts[pid] = start
	}
}

// judgedDead waits, for at most ten seconds, for the verdict on a lock whose
// holder has just been ended to reach VerdictDead. The Windows LockFileEx
// documentation says that the operating system unlocks a terminated
// process's locks but that "the time it takes ... depends upon available
// system resources", so the first judgement after the holder is waited on
// can still find the lock held. The loop ends on the first VerdictDead and
// fails on anything other than VerdictLive before it.
func judgedDead(t *testing.T, path string) LockRecord {
	t.Helper()
	began := time.Now()
	deadline := began.Add(10 * time.Second)
	for {
		record, verdict := JudgeLock(path)
		if verdict == VerdictDead {
			t.Logf("%s judged dead %v after its holder was waited on", path, time.Since(began))
			return record
		}
		if verdict != VerdictLive || time.Now().After(deadline) {
			t.Fatalf("the lock at %s judged %s after its holder ended, wanted dead", path, verdict)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
