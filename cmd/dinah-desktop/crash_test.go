package main

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

// A crash report the last run left is kept under its time and reported, and
// the next run starts with an empty report file.
func TestCaptureCrashesKeepsAndReportsTheLastCrash(t *testing.T) {
	a, _ := providerTestApp(t)
	// The runtime holds the report file open; let it go so the test's folder
	// can be removed.
	t.Cleanup(func() { _ = debug.SetCrashOutput(nil, debug.CrashOptions{}) })
	dir := logDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	report := "fatal error: concurrent map writes\n\ngoroutine 42 [running]:\n"
	if err := os.WriteFile(filepath.Join(dir, "crash.log"), []byte(report), 0o600); err != nil {
		t.Fatal(err)
	}
	kept := captureCrashes()
	if kept == "" || !strings.HasPrefix(filepath.Base(kept), "crash-") {
		t.Fatalf("the report was not kept: %q", kept)
	}
	if raw, _ := os.ReadFile(kept); string(raw) != report {
		t.Fatalf("the kept report is %q", raw)
	}
	if fi, err := os.Stat(filepath.Join(dir, "crash.log")); err != nil || fi.Size() != 0 {
		t.Fatalf("crash.log is not ready and empty: %v", err)
	}
	a.reportPreviousCrash(kept)
	items, err := a.Notifications()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range items {
		if n.Kind == "app" && n.Severity == "error" && strings.Contains(n.Body, "concurrent map writes") && strings.Contains(n.Body, kept) {
			found = true
		}
	}
	if !found {
		t.Fatalf("no notification of the crash: %+v", items)
	}
	if again := captureCrashes(); again != "" {
		t.Fatalf("an empty report was taken for a crash: %q", again)
	}
}
