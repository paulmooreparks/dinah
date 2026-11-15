package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"studio/internal/bus"
	"studio/internal/logicalchats"
)

func orgChartForbidProviderStarts(t *testing.T) *atomic.Int32 {
	t.Helper()
	var starts atomic.Int32
	previous := providerModules
	providerModules = map[string]providerModule{}
	for id, module := range previous {
		if module.Adapter != nil {
			adapter := *module.Adapter
			adapter.prepare = func(*App) (providerStart, error) {
				starts.Add(1)
				return providerStart{}, errors.New("org chart must not start a provider")
			}
			module.Adapter = &adapter
		}
		module.PrepareApplets = nil
		providerModules[id] = module
	}
	t.Cleanup(func() { providerModules = previous })
	return &starts
}

func TestOrgChartUsesCanonicalSnapshotWithoutProviderDiscovery(t *testing.T) {
	starts := orgChartForbidProviderStarts(t)
	a, connection := providerTestApp(t)
	ws := t.TempDir()
	model := logicalchats.Document{Version: logicalchats.Version, Revision: 1,
		Chats: map[string]logicalchats.Chat{
			"logical-chain": {ID: "logical-chain", Head: "claude:head", Title: "Current title", Workspace: ws,
				Members: []logicalchats.Member{{SessionID: "codex:old", Provider: "codex", State: "committed_predecessor"}, {SessionID: "claude:head", Provider: "claude", State: "active_head"}}},
			"logical-fork": {ID: "logical-fork", Head: "codex:fork", Title: "Separate fork", Workspace: ws,
				Members: []logicalchats.Member{{SessionID: "codex:fork", Provider: "codex", State: "active_head"}}},
			"logical-bin": {ID: "logical-bin", Head: "codex:bin", Title: "Binned", Workspace: ws,
				Members: []logicalchats.Member{{SessionID: "codex:bin", Provider: "codex", State: "active_head"}}},
		}, Aliases: map[string]string{"codex:old": "logical-chain", "claude:head": "logical-chain", "codex:fork": "logical-fork", "codex:bin": "logical-bin"}}
	a.settings.logical = &model
	a.settings.BusSessions = map[string]string{"codex:old": "window-old", "claude:head": "window-head", "codex:fork": "window-fork", "codex:bin": "window-bin"}
	a.settings.RemovedChats = map[string]chatRemoval{"codex:bin": {Stage: "future-reserved-stage"}}
	if err := a.busHost().With(func(b *bus.Bus) error {
		for _, id := range []string{"window-old", "window-head", "window-fork", "window-bin"} {
			if _, err := b.RegisterSession(context.Background(), id, ws, id); err != nil {
				return err
			}
		}
		_, err := b.SetLead(context.Background(), "window-fork", "window-old")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	before := len(connection.calls)
	view, err := a.OrgChart()
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Chats) != 2 || len(view.Teams) != 1 || view.Teams[0].LeadID != "logical-chain" || len(view.Teams[0].MemberIDs) != 1 || view.Teams[0].MemberIDs[0] != "logical-fork" {
		t.Fatalf("canonical inventory/edges changed: %+v", view)
	}
	if len(connection.calls) != before {
		t.Fatal("chart queried a provider")
	}
	if starts.Load() != 0 {
		t.Fatal("chart tried to start a provider")
	}
	for _, chat := range view.Chats {
		if chat.Resolution != "resolved" || chat.Workspace != ws {
			t.Fatalf("bad local metadata: %+v", chat)
		}
	}
	again, err := a.OrgChart()
	if err != nil || again.Generation != view.Generation {
		t.Fatalf("unchanged snapshot changed generation: %v", err)
	}
	changed := model.Chats["logical-chain"]
	changed.Title = "Changed title"
	model.Chats["logical-chain"] = changed
	again, err = a.OrgChart()
	if err != nil || again.Generation == view.Generation {
		t.Fatalf("changed snapshot kept generation: %v", err)
	}
}

func TestOrgChartKeepsRepairOwnershipExplicit(t *testing.T) {
	starts := orgChartForbidProviderStarts(t)
	a, connection := providerTestApp(t)
	a.settings.logical = &logicalchats.Document{Version: logicalchats.Version,
		Chats:   map[string]logicalchats.Chat{"repair-chat": {ID: "repair-chat", Head: "codex:uncertain", Title: "Needs repair", RepairCodes: []string{"ambiguous_ownership"}, Members: []logicalchats.Member{{SessionID: "codex:uncertain", Provider: "codex", State: "active_head"}}}},
		Aliases: map[string]string{"codex:uncertain": "repair-chat"}}
	view, err := a.OrgChart()
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Chats) != 1 || view.Chats[0].Resolution != "repair" || view.Chats[0].Reason == "" || view.Chats[0].Head != "" || len(view.Teams) != 0 {
		t.Fatalf("repair ownership guessed: %+v", view)
	}
	if len(connection.calls) != 0 {
		t.Fatal("repair query contacted provider")
	}
	if starts.Load() != 0 {
		t.Fatal("repair query tried to start a provider")
	}
}
