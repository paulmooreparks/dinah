package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"studio/internal/acp"
)

func TestSharedInventoryUnknownProgramRequiresRepair(t *testing.T) {
	starts := orgChartForbidProviderStarts(t)
	a, conn := providerTestApp(t)
	ws := t.TempDir()
	if err := a.updateSettings(func(s *Settings) error {
		s.ListedChats = map[string]listedChat{"unknown-native": {Provider: "invented-unknown", Workspace: ws, Title: "Safe unknown title"}}
		s.SessionProviders = map[string]string{"unknown-native": "invented-unknown"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	view, err := a.SavedChats()
	if err != nil || len(view.Chats) != 1 || view.Chats[0].Resolution != "repair" || view.Chats[0].Reason == "" || view.Chats[0].Title != "Safe unknown title" {
		t.Fatalf("unknown program must remain visible for repair: %+v %v", view, err)
	}
	listing, err := a.ListChatCleanup()
	if err != nil || len(listing.Rows) != 1 || listing.Rows[0].Eligible || listing.Rows[0].Refusal == "" {
		t.Fatalf("unknown program must not authorize cleanup: %+v %v", listing.Rows, err)
	}
	if starts.Load() != 0 || len(conn.calls.Snapshot()) != 0 {
		t.Fatal("unknown program repair contacted a provider")
	}
}

func TestSharedInventoryConflictingRefreshPersistsHoldAcrossRestart(t *testing.T) {
	for _, changedScope := range []bool{false, true} {
		t.Run(map[bool]string{false: "program", true: "workspace"}[changedScope], func(t *testing.T) {
			a, _ := providerTestApp(t)
			ws, other := t.TempDir(), t.TempDir()
			if err := a.recordListedSessions("devin", ws, []acp.SessionInfo{{SessionID: "held-conflict", Cwd: ws, Title: "Safe title"}, {SessionID: "keep", Cwd: ws, Title: "Keep"}}); err != nil {
				t.Fatal(err)
			}
			provider, scope := "claude", ws
			if changedScope {
				provider, scope = "devin", other
			}
			seq, err := a.beginListing(provider, scope)
			if err != nil {
				t.Fatal(err)
			}
			if err := a.finishListing(provider, scope, seq, []acp.SessionInfo{{SessionID: "held-conflict", Cwd: scope, Title: "Contradiction"}}, true); err == nil {
				t.Fatal("conflict accepted")
			}
			loaded, err := loadSettings()
			if err != nil {
				t.Fatal(err)
			}
			a.settings = loaded
			row := loaded.ListedChats["held-conflict"]
			if row.Repair == "" || len(row.Conflicts) != 1 || row.Title != "Safe title" || row.Provider != "devin" || row.Workspace != ws {
				t.Fatalf("lost conflict evidence: %+v", row)
			}
			if loaded.ListingScopes[listingScopeKey(provider, scope)].Complete {
				t.Fatal("contradictory refresh claimed completeness")
			}
			if err := a.recordListedSessions("devin", ws, []acp.SessionInfo{{SessionID: "held-conflict", Cwd: ws, Title: "Safe title"}}); err != nil {
				t.Fatal(err)
			}
			if a.settings.ListedChats["held-conflict"].Repair == "" || len(a.settings.ListedChats["held-conflict"].Conflicts) != 1 {
				t.Fatal("later refresh released repair hold")
			}
			if err := a.RemoveChat("held-conflict"); err == nil {
				t.Fatal("repair authorized deletion")
			}
		})
	}
}

func TestSharedInventoryLegacyAmbiguityRetainsBothObservations(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	if err := a.recordListedSessions("devin", ws, []acp.SessionInfo{{SessionID: "legacy", Cwd: ws, Title: "Sequenced title"}}); err != nil {
		t.Fatal(err)
	}
	legacy, err := a.settings.clone()
	if err != nil {
		t.Fatal(err)
	}
	legacy.ListedChats["legacy"] = listedChat{Provider: "devin", Workspace: ws, Title: "Legacy title", ObservedAt: "2026-11-15T14:00:00Z"}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir(), "settings.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadSettings()
	if err != nil {
		t.Fatal(err)
	}
	row := loaded.ListedChats["legacy"]
	if row.Repair == "" || row.Title != "Legacy title" || len(row.Conflicts) != 1 || row.Conflicts[0].Title != "Sequenced title" {
		t.Fatalf("lost legacy observations: %+v", row)
	}
	if err := saveSettings(loaded); err != nil {
		t.Fatal(err)
	}
	loaded, err = loadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if row := loaded.ListedChats["legacy"]; row.Repair == "" || len(row.Conflicts) != 1 {
		t.Fatalf("repair lost on rewrite: %+v", row)
	}
}

func TestSharedInventoryRetainsNativeTitlesWithoutProviderContact(t *testing.T) {
	starts := orgChartForbidProviderStarts(t)
	a, conn := providerTestApp(t)
	ws := t.TempDir()
	for _, provider := range []string{"devin", "claude", "ollama", "codex"} {
		if err := a.recordListedSessions(provider, ws, []acp.SessionInfo{{SessionID: provider + ":native", Cwd: ws, Title: provider + " native title"}}); err != nil {
			t.Fatal(err)
		}
	}
	view, err := a.SavedChats()
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Chats) != 3 {
		t.Fatalf("native population %+v", view.Chats)
	}
	for _, row := range view.Chats {
		if row.Provider == "codex" || row.Title != row.Provider+" native title" || row.Freshness != "lastKnown" || row.State != nil || row.Open || row.Cwd != ws || row.Resolution != "resolved" {
			t.Fatalf("native row %+v", row)
		}
	}
	if starts.Load() != 0 || len(conn.calls.Snapshot()) != 0 {
		t.Fatal("passive inventory contacted a provider")
	}
	if _, err := a.RenameSession("devin:native", "Person's name"); err != nil {
		t.Fatal(err)
	}
	after, err := a.SavedChats()
	if err != nil || after.Generation == view.Generation {
		t.Fatalf("rename generation %v", err)
	}
	listing, err := a.ListChatCleanup()
	if err != nil || len(listing.Rows) != len(after.Chats) {
		t.Fatalf("cleanup population %+v %v", listing.Rows, err)
	}
	for _, row := range listing.Rows {
		if row.SessionID == "devin:native" && row.Title != "Person's name" {
			t.Fatal("cleanup title disagrees")
		}
	}
	chart, err := a.OrgChart()
	if err != nil || len(chart.Chats) != len(after.Chats) {
		t.Fatalf("chart population %+v %v", chart, err)
	}
	for _, chat := range chart.Chats {
		found := false
		for _, row := range after.Chats {
			if chat.Head == row.ID {
				found = chat.ChatID == row.ChatID && chat.Title == row.Title && chat.Workspace == row.Cwd && chat.Provider == row.Provider && chat.State == nil && row.State == nil
			}
		}
		if !found {
			t.Fatalf("chart projection disagrees %+v", chat)
		}
	}
	if a.settings.ListedChats["devin:native"].Title != "devin native title" {
		t.Fatal("override erased provider provenance")
	}
}

func TestSharedInventoryScopesCompletenessAndRefreshOrdering(t *testing.T) {
	a, _ := providerTestApp(t)
	ws, other := t.TempDir(), t.TempDir()
	rows := []acp.SessionInfo{{SessionID: "a", Cwd: ws, Title: "A"}, {SessionID: "b", Cwd: ws, Title: "B"}}
	if err := a.recordListedSessions("devin", ws, rows); err != nil {
		t.Fatal(err)
	}
	if err := a.recordListedSessions("devin", other, []acp.SessionInfo{{SessionID: "other", Cwd: other, Title: "Other"}}); err != nil {
		t.Fatal(err)
	}
	old, _ := a.beginListing("devin", ws)
	newer, _ := a.beginListing("devin", ws)
	if err := a.finishListing("devin", ws, newer, rows[:1], true); err != nil {
		t.Fatal(err)
	}
	if a.settings.ListedChats["b"].Freshness != "missing" || a.settings.ListedChats["other"].Freshness != "lastKnown" {
		t.Fatal("missing escaped completed scope")
	}
	rows[0].Title = "Stale title"
	if err := a.finishListing("devin", ws, old, rows, true); err != nil {
		t.Fatal(err)
	}
	if a.settings.ListedChats["a"].Title != "A" || a.settings.ListedChats["b"].Freshness != "missing" {
		t.Fatal("old response replaced newer observation")
	}
	partial, _ := a.beginListing("devin", ws)
	if err := a.finishListing("devin", ws, partial, rows[1:], false); err != nil {
		t.Fatal(err)
	}
	if a.settings.ListedChats["a"].Freshness != "lastKnown" || a.settings.ListedChats["b"].Freshness != "lastKnown" {
		t.Fatal("partial listing inferred absence")
	}
	invalid, _ := a.beginListing("devin", ws)
	if err := a.finishListing("devin", ws, invalid, []acp.SessionInfo{{SessionID: "wrong", Cwd: other}}, true); err == nil {
		t.Fatal("invalid listing scope accepted")
	}
	if a.settings.ListedChats["a"].Freshness != "lastKnown" {
		t.Fatal("invalid listing marked a row missing")
	}
	view, err := a.SavedChats()
	if err != nil || len(view.Chats) != 3 {
		t.Fatalf("missing rows disappeared %+v %v", view, err)
	}
}

func TestSharedInventorySurvivesOlderBuildDroppingCacheFields(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	if err := a.recordListedSessions("claude", ws, []acp.SessionInfo{{SessionID: "claude:kept", Cwd: ws, Title: "Safe provider title"}}); err != nil {
		t.Fatal(err)
	}
	first, err := loadSettings()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(configDir(), "settings.json")
	data, err := readStateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var old map[string]json.RawMessage
	if err := json.Unmarshal(data, &old); err != nil {
		t.Fatal(err)
	}
	delete(old, "listedChats")
	delete(old, "listingScopes")
	data, _ = json.Marshal(old)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := loadSettings()
	if err != nil || !reflect.DeepEqual(second.ListedChats["claude:kept"], first.ListedChats["claude:kept"]) {
		t.Fatalf("older write erased inventory %+v %v", second.ListedChats, err)
	}
	a.settings = second
	view, err := a.SavedChats()
	if err != nil || len(view.Chats) != 1 || view.Chats[0].Title != "Safe provider title" {
		t.Fatalf("second start %+v %v", view, err)
	}
	if err := os.WriteFile(filepath.Join(configDir(), "chat-inventory.json"), []byte("damaged"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSettings(); err == nil {
		t.Fatal("corrupt inventory silently fell back")
	}
}

func TestSharedInventoryMigratesExistingCacheBeforeFirstSave(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	legacy := a.settings
	legacy.ListedChats = map[string]listedChat{"native": {Provider: "devin", Workspace: ws, Title: "Previously listed"}}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir(), "settings.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := loadSettings()
	if err != nil || first.ListedChats["native"].Title != "Previously listed" {
		t.Fatalf("first new start %v", err)
	}
	legacy.ListedChats = nil
	data, _ = json.Marshal(legacy)
	if err := os.WriteFile(filepath.Join(configDir(), "settings.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := loadSettings()
	if err != nil || second.ListedChats["native"].Title != "Previously listed" {
		t.Fatalf("first read failed to preserve migration %v", err)
	}
}

func TestSharedInventoryKeepsRepairAndFutureReservation(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	if err := a.recordListedSessions("devin", ws, []acp.SessionInfo{{SessionID: "held", Cwd: ws, Title: "Held"}, {SessionID: "conflict", Cwd: ws, Title: "Conflict"}}); err != nil {
		t.Fatal(err)
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.RemovedChats = map[string]chatRemoval{"held": {Stage: "future-phase"}}
		s.SessionProviders = map[string]string{"conflict": "claude"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	view, err := a.SavedChats()
	if err != nil || len(view.Chats) != 1 || view.Chats[0].Resolution != "repair" || view.Chats[0].Reason == "" {
		t.Fatalf("repair/hold %+v %v", view, err)
	}
	if err := a.recordListedSessions("devin", ws, []acp.SessionInfo{{SessionID: "held", Cwd: ws, Title: "New title"}}); err != nil {
		t.Fatal(err)
	}
	if a.settings.ListedChats["held"].Title != "Held" {
		t.Fatal("refresh changed reserved metadata")
	}
}

func TestSharedInventoryRetainsRuntimeTitleWithSeparateProvenance(t *testing.T) {
	a, conn := providerTestApp(t)
	ws := t.TempDir()
	id := "codex:already-admitted-runtime"
	chat := newSessionState(&sessionSeed{ID: id, Provider: "codex", Cwd: ws, Title: "Observed runtime title"}, sessionInitial{Open: true})
	a.runtime.PutConversationLocked(id, chat)
	if _, err := a.SavedChats(); err != nil {
		t.Fatal(err)
	}
	recorded := a.settings.ListedChats[id]
	if recorded.Title != "" || recorded.LocalTitle != "Observed runtime title" || !recorded.LocalObservation {
		t.Fatalf("runtime provenance %+v", recorded)
	}
	loaded, err := loadSettings()
	if err != nil {
		t.Fatal(err)
	}
	a.runtime.RemoveConversationLocked(id)
	a.settings = loaded
	view, err := a.SavedChats()
	if err != nil || len(view.Chats) != 1 || view.Chats[0].Title != "Observed runtime title" || view.Chats[0].Open {
		t.Fatalf("runtime metadata lost across reconstruction %+v %v", view, err)
	}
	if len(conn.calls.Snapshot()) != 0 {
		t.Fatal("runtime capture contacted provider")
	}
}

func TestSharedInventoryProjectionConflictProducesRepairRow(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	if err := a.recordListedSessions("devin", ws, []acp.SessionInfo{{SessionID: "conflict", Cwd: ws, Title: "Keep title"}}); err != nil {
		t.Fatal(err)
	}
	old, err := a.settings.clone()
	if err != nil {
		t.Fatal(err)
	}
	changed := old.ListedChats["conflict"]
	changed.Provider = "claude"
	old.ListedChats["conflict"] = changed
	data, _ := json.Marshal(old)
	if err := os.WriteFile(filepath.Join(configDir(), "settings.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadSettings()
	if err != nil {
		t.Fatal(err)
	}
	a.settings = loaded
	view, err := a.SavedChats()
	if err != nil || len(view.Chats) != 1 || view.Chats[0].Resolution != "repair" || view.Chats[0].Reason == "" {
		t.Fatalf("conflict did not become a repair row %+v %v", view, err)
	}
	if err := a.RemoveChat("conflict"); err == nil {
		t.Fatal("conflicting cached ownership authorized deletion")
	}
}
