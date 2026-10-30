package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"studio/internal/applet/broker"
)

func readLog(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(logDir(), "dinah.log"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(data)
}

// The log keeps what its level says, one line for each entry with its
// source, and records applet actions under their applet.
func TestLog(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := &App{}
	t.Cleanup(func() { setLogLevel(LogProblems) })

	setLogLevel(LogProblems)
	logf("core", "info", "not kept")
	logf("core", "warning", "kept\nacross lines")
	if got := readLog(t); strings.Contains(got, "not kept") || !strings.Contains(got, "WARN") || !strings.Contains(got, "[core] kept | across lines") || strings.Count(got, "\n") != 1 {
		t.Fatalf("problems: %q", got)
	}

	setLogLevel(LogAll)
	if err := a.Log("script:backup", "info", "copied 3 files"); err != nil {
		t.Fatal(err)
	}
	if err := a.Log("Not A Source", "info", "x"); err == nil {
		t.Fatal("a source of the wrong shape was taken")
	}
	if err := a.Log("terminal", "debug", "x"); err == nil {
		t.Fatal("a level of the wrong kind was taken")
	}
	now := time.Now()
	logAppletEvent("applet:invocation", broker.Invocation{Action: "dinah.projects/find@1", OwnerKind: "applet", OwnerID: "dinah.terminal", Status: "failed", Error: "no such code", Created: now, Updated: now})
	logAppletEvent("applet:invocation", broker.Invocation{Action: "parks.changelog/summarize@1", OwnerKind: "chat", OwnerID: "s1", Status: "done", Created: now, Updated: now.Add(1500 * time.Millisecond)})
	got := readLog(t)
	for _, want := range []string{"INFO  [script:backup] copied 3 files", "ERROR [applet:dinah.projects] dinah.projects/find@1 failed for applet dinah.terminal: no such code", "[applet:parks.changelog] parks.changelog/summarize@1 done for chat s1 in 1.5s"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}

	setLogLevel(LogOff)
	logf("core", "error", "dropped")
	if strings.Contains(readLog(t), "dropped") {
		t.Fatal("the log kept an entry while off")
	}
}

// A log past its size becomes dinah.1.log, and the oldest beyond
// logKeep files is removed.
func TestLogRotation(t *testing.T) {
	isolateWorkspaceSettings(t)
	t.Cleanup(func() { setLogLevel(LogProblems) })
	setLogLevel(LogAll)
	dir := logDir()
	os.MkdirAll(dir, 0o700)
	big := strings.Repeat("x", logMaxSize)
	for i := 1; i < logKeep; i++ {
		os.WriteFile(filepath.Join(dir, "dinah."+string(rune('0'+i))+".log"), []byte("old "+string(rune('0'+i))), 0o600)
	}
	os.WriteFile(filepath.Join(dir, "dinah.log"), []byte(big), 0o600)
	logf("core", "info", "after the roll")
	if got := readLog(t); !strings.Contains(got, "after the roll") || len(got) > 200 {
		t.Fatalf("the new log is %d bytes", len(got))
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "dinah.1.log")); len(data) != logMaxSize {
		t.Fatal("the full log did not become dinah.1.log")
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "dinah.3.log")); string(data) != "old 2" {
		t.Fatalf("the older logs did not move up: %q", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "dinah.4.log")); !os.IsNotExist(err) {
		t.Fatal("more than logKeep files are kept")
	}
}

// When the app ends its logs take no more: an entry made after they are closed, by a goroutine
// still running, writes nothing into the folder, which a test or an uninstaller may be removing.
func TestNothingIsWrittenToTheLogsAfterTheyAreClosed(t *testing.T) {
	a, _ := providerTestApp(t)
	setLogLevel(LogAll)
	logf("core", "info", "before the close")
	a.chatLogs().Begin("chat-x", "codex", "ws", "Title")
	a.closeAgentLogs()
	path := filepath.Join(logDir(), product.LogFile)
	before, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(before), "before the close") {
		t.Fatalf("the entry made before the close was not written: %v %q", err, before)
	}
	logf("core", "info", "after the close")
	a.chatLogs().Begin("chat-y", "codex", "ws", "Title")
	a.chatLogs().Note("chat-x", "after the close")
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatalf("an entry was written after the close: %q", after)
	}
	if _, err := os.Stat(a.chatLogs().Path("chat-y")); !os.IsNotExist(err) {
		t.Fatalf("a chat log was made after the close: %v", err)
	}
	// A new start in the same folder logs again.
	openAppLog()
	logf("core", "info", "after the restart")
	if data, _ := os.ReadFile(path); !strings.Contains(string(data), "after the restart") {
		t.Fatalf("the log did not open again: %q", data)
	}
}
