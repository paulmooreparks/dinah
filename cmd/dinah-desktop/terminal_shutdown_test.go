package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"studio/internal/terminals"
)

func terminalShutdownApp(t *testing.T) *App {
	t.Helper()
	isolateWorkspaceSettings(t)
	openAppLog()
	appLog.Lock()
	previous := appLog.level
	appLog.Unlock()
	t.Cleanup(func() { closeAppLog(); setLogLevel(previous) })
	return &App{}
}

func TestTerminalShutdownUsesFreshTotalBudgetAndPermitsCleanUpdate(t *testing.T) {
	a := terminalShutdownApp(t)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	a.ctx = canceled
	a.update = &updateWatch{path: "candidate", restarting: true}
	before := time.Now()
	a.shutdownTerminals(func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		if !ok || ctx.Err() != nil || deadline.Before(before.Add(terminalShutdownBudget)) || deadline.After(time.Now().Add(terminalShutdownBudget)) {
			t.Fatalf("invalid fresh budget: %v %v", deadline, ctx.Err())
		}
		return nil
	})
	launched := false
	a.startUpdateUsing(func(path string, args ...string) error {
		launched = path == "candidate" && len(args) == 2 && args[0] == "--after"
		return nil
	})
	if !launched {
		t.Fatal("clean teardown suppressed update")
	}
}

func TestTerminalShutdownRetainsDiagnosticsNoticeAndStickyUpdateSuppression(t *testing.T) {
	a := terminalShutdownApp(t)
	setLogLevel(LogOff)
	a.update = &updateWatch{path: "candidate", restarting: true, pending: true}
	a.shutdownTerminals(func(context.Context) error {
		return &terminals.ShutdownError{Cause: context.DeadlineExceeded, Pending: []terminals.PendingTerminal{{ID: "term-known", PID: 42, OutputPending: true, ReadError: "read failed"}}}
	})
	data, err := readRegularStateFile(filepath.Join(configDir(), "terminal-shutdown.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record terminalShutdownRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if len(record.Pending) != 1 || record.Pending[0].ID != "term-known" || record.Pending[0].PID != 42 || !record.Pending[0].OutputPending || record.Pending[0].ReadError != "read failed" {
		t.Fatalf("lost stages: %+v", record)
	}
	if _, err := os.Stat(filepath.Join(logDir(), record.ID+".json")); err != nil {
		t.Fatal("incident missing with logging off", err)
	}
	if a.update.restarting || a.update.pending {
		t.Fatal("restart flags retained")
	}
	// Later success cannot revoke suppression for this teardown.
	a.shutdownTerminals(func(context.Context) error { return nil })
	a.update.restarting = true
	a.startUpdateUsing(func(string, ...string) error { t.Fatal("incomplete teardown launched update"); return nil })
	b := &App{}
	b.restoreTerminalShutdownNotice()
	notices, err := b.Notifications()
	if err != nil || len(notices) != 1 || notices[0].ID != record.ID || !strings.Contains(notices[0].Body, "Restart to Update was skipped") {
		t.Fatalf("next launch notice: %+v %v", notices, err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, exe, "-test.run=^TestTerminalShutdownNoticeChild$")
	child.Env = append(os.Environ(), "DINAH_TEST_TERMINAL_NOTICE_DIR="+configDir(), "DINAH_TEST_TERMINAL_NOTICE_ID="+record.ID)
	if out, err := child.CombinedOutput(); err != nil {
		t.Fatalf("notice in new process: %v\n%s", err, out)
	}
	if err := b.DismissNotification(record.ID); err != nil {
		t.Fatal(err)
	}
	b.restoreTerminalShutdownNotice()
	notices, err = b.Notifications()
	if err != nil || len(notices) != 0 {
		t.Fatalf("dismissed notice resurrected: %+v %v", notices, err)
	}
}

func TestTerminalShutdownNoticeChild(t *testing.T) {
	dir := os.Getenv("DINAH_TEST_TERMINAL_NOTICE_DIR")
	if dir == "" {
		t.Skip("subprocess helper")
	}
	configOverride = dir
	openAppLog()
	defer closeAppLog()
	a := &App{}
	a.restoreTerminalShutdownNotice()
	notices, err := a.Notifications()
	if err != nil || len(notices) != 1 || notices[0].ID != os.Getenv("DINAH_TEST_TERMINAL_NOTICE_ID") {
		t.Fatalf("persistent shutdown notice: %+v %v", notices, err)
	}
}

func TestTerminalShutdownRetriesNoticeFailureOnNextLaunch(t *testing.T) {
	a := terminalShutdownApp(t)
	path := filepath.Join(configDir(), "notifications.json")
	if err := os.WriteFile(path, []byte("invalid store"), 0o600); err != nil {
		t.Fatal(err)
	}
	a.shutdownTerminals(func(context.Context) error {
		return errors.New(strings.Repeat("x", 20000) + " incomplete synthetic shutdown")
	})
	data, err := readRegularStateFile(filepath.Join(configDir(), "terminal-shutdown.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record terminalShutdownRecord
	if err := json.Unmarshal(data, &record); err != nil || record.NoticeError == "" {
		t.Fatalf("notice failure not recorded: %+v %v", record, err)
	}
	if len(record.Error) < 20000 {
		t.Fatal("full diagnostic error was truncated")
	}
	// The original failure is retained; the test repairs only its own fixture.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	b := &App{}
	b.restoreTerminalShutdownNotice()
	notices, err := b.Notifications()
	if err != nil || len(notices) != 1 || notices[0].ID != record.ID {
		t.Fatalf("notice recovery: %+v %v", notices, err)
	}
}
