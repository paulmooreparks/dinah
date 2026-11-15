package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCleanupSavedSettingsExcludeOtherChatsAndWorkspaceRecords(t *testing.T) {
	s := Settings{
		SessionTitles:       map[string]string{"selected": "Keep this title", "separate": "Unrelated title"},
		SessionProviders:    map[string]string{"selected": "codex", "separate": "claude"},
		SessionOptions:      map[string]map[string]json.RawMessage{"selected": {"model": json.RawMessage(`"example"`)}, "separate": {"model": json.RawMessage(`"other"`)}},
		BusSessions:         map[string]string{"selected": "window-one", "separate": "window-two"},
		WorkspaceRecords:    []WorkspaceRecord{{ID: "unrelated-workspace", Name: "Shared documents"}},
		Templates:           []Template{{Name: "Not chat-owned", Text: "Leave this alone"}},
		ChatTransferHistory: []chatTransfer{{Source: "selected", Target: "old-candidate"}, {Source: "separate", Target: "another"}},
	}
	saved := cleanupSavedSettings(s, []string{"selected", "old-candidate"})
	if len(saved.SessionTitles) != 1 || saved.SessionTitles["selected"] != "Keep this title" || len(saved.SessionOptions) != 1 {
		t.Fatalf("chat snapshot %+v", saved)
	}
	data, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("Shared documents")) || bytes.Contains(data, []byte("Not chat-owned")) || len(saved.ChatTransferHistory) != 1 {
		t.Fatal("snapshot retained unrelated ownership")
	}
	saved.SessionOptions["selected"]["model"] = json.RawMessage(`"changed"`)
	if string(s.SessionOptions["selected"]["model"]) != `"example"` {
		t.Fatal("snapshot mutated active settings")
	}
}

func TestCleanupPermanentTombstoneFiltersStaleComposerSave(t *testing.T) {
	a, _ := providerTestApp(t)
	state := UIState{Version: 1, Values: map[string]json.RawMessage{
		"dinah-draft:chat:old":     json.RawMessage(`"old composer"`),
		"dinah-draft:chat:head":    json.RawMessage(`"current composer"`),
		"unrelated-document-draft": json.RawMessage(`{"text":"keep"}`),
	}}
	if err := a.SaveUIState(state); err != nil {
		t.Fatal(err)
	}
	a.settings.RemovedChats = map[string]chatRemoval{"old": {Stage: "retained"}, "head": {Stage: "retained"}}
	if err := a.SaveUIState(state); err != nil {
		t.Fatal(err)
	}
	got, err := a.ReadUIState()
	if err != nil || len(got.Values) != 3 {
		t.Fatal("staging lost drafts", got, err)
	}
	a.settings.RemovedChats["old"] = chatRemoval{Stage: "deleted"}
	a.settings.RemovedChats["head"] = chatRemoval{Stage: "deleted"}
	if err := a.SaveUIState(state); err != nil {
		t.Fatal(err)
	}
	got, err = a.ReadUIState()
	if err != nil || len(got.Values) != 1 || got.Values["unrelated-document-draft"] == nil {
		t.Fatal("stale save revived composer or lost unrelated data", got, err)
	}
	if len(state.Values) != 3 {
		t.Fatal("save mutated caller's snapshot")
	}
}
