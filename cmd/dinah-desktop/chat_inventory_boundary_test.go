package main

import (
	"testing"

	"studio/internal/sessionhost"
	"studio/internal/workspaces"
)

func TestSavedChatsAcceptsUnrelatedSettingsDuringRead(t *testing.T) {
	a, _, ws := orgActionsFixture(t)
	if err := a.updateSettings(func(s *Settings) error {
		s.ListedChats = map[string]listedChat{"codex:a": {Provider: "codex", Workspace: ws}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	_, err := a.savedChatsAtBoundary(func() {
		if err := a.updateSettings(func(s *Settings) error {
			s.ShowThinking = !s.ShowThinking
			baseline := int64(123)
			s.MentionBaseline = &baseline
			s.WorkspaceRecords[0].Pins = append(s.WorkspaceRecords[0].Pins, workspaces.Pin{Kind: "chat", Session: "codex:a"})
			row := s.ListedChats["codex:a"]
			row.ObservedAt, row.UpdatedAt, row.Freshness, row.Sequence = "2026-11-16T01:00:00Z", "2026-11-16T01:00:00Z", "current", 5
			s.ListedChats["codex:a"] = row
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil {
		t.Fatal("unrelated preferences and message cursor invalidated chat ownership", err)
	}
}

func TestChatInventoryAcceptsStatePublicationDuringRead(t *testing.T) {
	a, lead, _, _, _, _ := teamApp(t)
	a.sendTeamStates()
	a.sendChatStates()
	holdInventoryPublicationTimers(t, a)
	publication := func() {
		before := lead.InfoVersioned()
		lead.SetState(&ChatState{Life: lifeOpen, Exec: &sessionhost.ChatExec{State: "working", Why: "Invented work"}}, &sessionhost.StateCounts{Working: 1})
		after := lead.InfoVersioned()
		if inventoryRuntimeHash(map[string]SessionInfo{lead.ID(): before}) == inventoryRuntimeHash(map[string]SessionInfo{lead.ID(): after}) {
			t.Fatal("the control did not publish a different runtime snapshot")
		}
	}
	if _, err := a.savedChatsAtBoundary(publication); err != nil {
		t.Fatal("execution publication invalidated inventory", err)
	}
	// Force another distinct publication between the two complete inventories.
	reset := func() { lead.SetState(&ChatState{Life: lifeOpen}, nil) }
	reset()
	if _, err := a.orgChartAtBoundary(publication); err != nil {
		t.Fatal("execution publication invalidated chart", err)
	}
	reset()
	if _, err := a.listChatCleanupAtBoundary(publication); err != nil {
		t.Fatal("execution publication invalidated cleanup", err)
	}
}

func TestInventoryIdentityStillGuardsOwnershipMetadata(t *testing.T) {
	a, _, _ := orgActionsFixture(t)
	a.mu.Lock()
	original, err := a.settings.clone()
	a.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Settings){
		"title":     func(s *Settings) { s.SessionTitles["codex:a"] = "Changed" },
		"provider":  func(s *Settings) { s.SessionProviders["codex:a"] = "claude" },
		"recipient": func(s *Settings) { s.BusSessions["codex:a"] = "another" },
		"binding":   func(s *Settings) { s.ChatBindings = map[string]string{"codex:a": "codex:b"} },
		"hidden":    func(s *Settings) { s.Hidden = append(s.Hidden, "codex:a") },
		"workspace": func(s *Settings) {
			row := s.ListedChats["codex:a"]
			row.Workspace += "-changed"
			s.ListedChats["codex:a"] = row
		},
		"repair": func(s *Settings) {
			row := s.ListedChats["codex:a"]
			row.Repair = "Changed ownership evidence"
			s.ListedChats["codex:a"] = row
		},
	} {
		t.Run(name, func(t *testing.T) {
			copy, err := original.clone()
			if err != nil {
				t.Fatal(err)
			}
			if copy.ListedChats == nil {
				copy.ListedChats = map[string]listedChat{}
			}
			change(&copy)
			if savedChatsSettingsStamp(copy) == savedChatsSettingsStamp(original) {
				t.Fatal("ownership change escaped the guard")
			}
		})
	}
}
