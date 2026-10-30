package main

import (
	"strings"
	"testing"
)

// A debug entry is kept only at the debug level, which has no control and is
// set in the settings file, so that work such as a pane's waits for a change
// does not fill a log that keeps everything else.
func TestDebugEntriesAreKeptOnlyAtTheDebugLevel(t *testing.T) {
	isolateWorkspaceSettings(t)
	t.Cleanup(func() { setLogLevel(LogProblems) })
	for _, level := range []string{LogOff, LogProblems, LogAll} {
		setLogLevel(level)
		logf("applet:test", "debug", "a pane's wait ended at %s", level)
	}
	if got := readLog(t); strings.Contains(got, "a pane's wait ended") {
		t.Fatalf("a debug entry was kept below the debug level: %q", got)
	}
	setLogLevel(LogDebug)
	logf("applet:test", "debug", "a pane's wait ended")
	logf("core", "info", "an info entry")
	got := readLog(t)
	if !strings.Contains(got, "DEBUG [applet:test] a pane's wait ended") || !strings.Contains(got, "an info entry") {
		t.Fatalf("the debug level: %q", got)
	}
	if !validLogLevel(LogDebug) {
		t.Fatal("the debug level is not accepted from the settings file")
	}
}
