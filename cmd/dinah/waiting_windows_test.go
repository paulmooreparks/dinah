//go:build windows

package main

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"dinah/internal/bench"
	"dinah/internal/durable"
)

// TestTheCommandLinePrintsAWaitToStandardError asserts that a move whose
// journal another process holds, sharing read only, past a 100 ms budget,
// waits after its anchor write and prints each notice to standard error,
// prefixed dinah: and naming the journal relative to the workbench, and
// then answers ok once the handle closes.
func TestTheCommandLinePrintsAWaitToStandardError(t *testing.T) {
	root := newBench(t)
	mustRun(t, root, "add", "waiting")
	anchor := strings.TrimSpace(mustRun(t, root, "path", "fx-1").out)
	journal := filepath.Join(filepath.Dir(anchor), bench.JournalName)
	name, err := windows.UTF16PtrFromString(journal)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("hold the journal: %v", err)
	}
	var once sync.Once
	closeIt := func() { once.Do(func() { windows.CloseHandle(handle) }) }
	t.Cleanup(closeIt)
	saved := durable.RetryBudget
	durable.RetryBudget = 100 * time.Millisecond
	durable.Waiting = func(wait durable.Wait) {
		forwardWaiting(wait)
		if wait.Notice == 2 {
			closeIt()
		}
	}
	t.Cleanup(func() {
		durable.RetryBudget = saved
		durable.Waiting = nil
	})
	got := runCLI(t, root, "move", "fx-1", "doing")
	if got.code != 0 {
		t.Fatalf("the waiting move answered %d %s", got.code, got.errw)
	}
	notices := 0
	for _, line := range strings.Split(got.errw, "\n") {
		if strings.HasPrefix(line, "dinah: waiting for cards/") && strings.Contains(line, "/journal.ndjson") {
			notices++
		}
	}
	if notices < 2 {
		t.Errorf("standard error carried %d wait notices, wanted at least two: %q", notices, got.errw)
	}
}
