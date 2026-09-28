package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
)

// TestCheckReportsADeadLockAndTheFinishClearsIt asserts the whole command
// line path of a lock whose holder is proven dead: check prints the
// check.stale-lock finding with its next step, check --finish prints the lock
// it cleared naming the dead holder and its pid, and the card's journal then
// shows the lock_reclaimed line carrying the dead record.
//
// The lock is planted with this test process's own identity, which run shares
// because it runs in this process, and with no acquisition in the process
// owning it. That is this process's own orphan, which the verdict proves dead
// without a second process, the way a release whose deletion failed leaves one.
func TestCheckReportsADeadLockAndTheFinishClearsIt(t *testing.T) {
	root := newBench(t)
	mustRun(t, root, "add", "held")
	anchor := strings.TrimSpace(mustRun(t, root, "path", "fx-1").out)
	dir := filepath.Dir(anchor)
	lockPath := filepath.Join(dir, bench.LockName)
	taken, err := bench.Acquire(dir, "brin", "2026-09-28T00:00:00Z")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	record, _ := bench.ReadLockRecord(lockPath)
	taken.Release()
	line, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(lockPath, append(line, '\n'), 0o644); err != nil {
		t.Fatalf("plant the orphan: %v", err)
	}

	checked := runCLI(t, root, "check")
	finding := "a lock stands whose holder is not shown to be running: brin dead; that process has ended, and `dinah check --finish` clears the lock"
	if !strings.Contains(checked.out, finding) {
		t.Errorf("check printed %q, wanted the stale-lock finding with its next step", checked.out)
	}

	finished := runCLI(t, root, "check", "--finish")
	cleared := "cleared " + lockPath + " (brin, pid " + strconv.Itoa(record.PID) + ", ended)"
	if !strings.Contains(finished.out, cleared) {
		t.Errorf("check --finish printed %q, wanted %q", finished.out, cleared)
	}
	if bench.Exists(lockPath) {
		t.Error("the finish left the dead lock standing")
	}

	journal := mustRun(t, root, "list", "fx-1/journal").out
	if !strings.Contains(journal, "lock reclaimed") || !strings.Contains(journal, `"actor":"brin"`) {
		t.Errorf("the journal reads %q, wanted a lock_reclaimed line carrying the dead record", journal)
	}
}
