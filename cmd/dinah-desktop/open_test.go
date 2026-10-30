package main

import (
	"os"
	"path/filepath"
	"testing"

	"studio/internal/agents"
)

// TestProtectedPath checks that the agent may write wherever its approvals
// let it, except the folders that hold the approval rules.
func TestProtectedPath(t *testing.T) {
	cfg := configDir()
	base, _ := os.UserConfigDir()
	home, _ := os.UserHomeDir()
	cases := map[string]bool{
		filepath.Join(home, "OneDrive", "Documents", "travel", "trip.md"): false,
		filepath.Join(cfg, "settings.json"):                               true,
		filepath.Join(cfg, "devin-all.json"):                              true,
		filepath.Join(base, "devin", "config.json"):                       true,
		filepath.Join(home, ".config", "devin", "config.json"):            true,
	}
	for p, closed := range cases {
		if got := agents.ProtectedPath(stateLayout(), p) != ""; got != closed {
			t.Errorf("%s: closed %v, want %v", p, got, closed)
		}
	}
}
