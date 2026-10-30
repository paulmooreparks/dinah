package main

import (
	"os"
	"testing"

	"studio/internal/agents"
	"studio/internal/layout"
)

func TestDiscoveryUsesCurrentWindowsPath(t *testing.T) {
	// Read-only opt-in check against an installed executable. No process or
	// model session is started, and the saved Windows environment is untouched.
	expected := os.Getenv("DINAH_DISCOVERY_EXECUTABLE")
	if expected == "" {
		t.Skip("set DINAH_DISCOVERY_EXECUTABLE for installed PATH discovery verification")
	}
	t.Setenv("PATH", t.TempDir())
	got := agents.FindExecutable("codex", nil)
	wantInfo, err := os.Stat(expected)
	if err != nil {
		t.Fatal(err)
	}
	gotInfo, err := os.Stat(got)
	if err != nil || !os.SameFile(wantInfo, gotInfo) {
		t.Fatalf("current Windows PATH discovery: got %q, want %q (%v)", got, expected, err)
	}
	if selected, err := agents.CheckExecutable(expected); err != nil || !layout.SamePath(selected, got) {
		t.Fatalf("explicit installer path failed: %q, %v", selected, err)
	}
}
