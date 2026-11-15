package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"studio/internal/acp"
)

func TestReleaseInventoryReviewFreshConflictRequiresRepair(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	if err := a.recordListedSessions("devin", ws, []acp.SessionInfo{{SessionID: "native-conflict", Cwd: ws, Title: "Safe known title"}}); err != nil {
		t.Fatal(err)
	}
	if err := a.recordListedSessions("claude", ws, []acp.SessionInfo{{SessionID: "native-conflict", Cwd: ws, Title: "Conflicting observation"}}); err == nil {
		t.Fatal("fixture did not reach conflicting listing")
	}
	view, err := a.SavedChats()
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Chats) != 1 || view.Chats[0].Resolution != "repair" || view.Chats[0].Reason == "" {
		t.Fatalf("observed conflict left an actionable row: %+v", view.Chats)
	}
}

func TestReleaseInventoryReviewOlderWriterDoesNotSilentlyRevertTitle(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	if err := a.recordListedSessions("devin", ws, []acp.SessionInfo{{SessionID: "native-legacy", Cwd: ws, Title: "Earlier observation"}}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(configDir(), "settings.json")
	data, err := readStateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var legacy map[string]json.RawMessage
	if err := json.Unmarshal(data, &legacy); err != nil {
		t.Fatal(err)
	}
	// Simulate the shipped native-cache writer that predates sequence and scope fields.
	row := map[string]string{"provider": "devin", "workspace": ws, "title": "Newer observation from older build", "observedAt": "2026-11-15T14:00:00Z"}
	legacy["listedChats"], err = json.Marshal(map[string]any{"native-legacy": row})
	if err != nil {
		t.Fatal(err)
	}
	delete(legacy, "listingScopes")
	data, err = json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadSettings()
	if err != nil {
		t.Fatal(err)
	}
	got := loaded.ListedChats["native-legacy"]
	if got.Title != row["title"] && got.Repair == "" {
		t.Fatalf("newer legacy observation silently replaced by stale projection: title=%q repair=%q", got.Title, got.Repair)
	}
}

func TestReleaseInventoryReviewRepairRetainsSafeKnownTitle(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	if err := a.recordListedSessions("devin", ws, []acp.SessionInfo{{SessionID: "native-repair", Cwd: ws, Title: "Safe known title"}}); err != nil {
		t.Fatal(err)
	}
	if err := a.updateSettings(func(s *Settings) error {
		if s.SessionProviders == nil {
			s.SessionProviders = map[string]string{}
		}
		s.SessionProviders["native-repair"] = "claude"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	view, err := a.SavedChats()
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Chats) != 1 || view.Chats[0].Resolution != "repair" {
		t.Fatalf("fixture did not reach repair: %+v", view.Chats)
	}
	if view.Chats[0].Title != "Safe known title" {
		t.Fatalf("repair erased known safe title: %q", view.Chats[0].Title)
	}
}
