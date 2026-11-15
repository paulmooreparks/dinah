package main

import (
	"context"
	"errors"
	"studio/internal/bus"
	"sync/atomic"
	"testing"
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

// seedOrgChartFixture writes local records without opening a provider.
func seedOrgChartFixture(t *testing.T, a *App, ws string) {
	t.Helper()
	ids := []string{"codex:old", "claude:head", "codex:fork", "codex:bin"}
	record := newWorkspaceRecord(ws)
	record.Name = "Invented Studio"
	for _, id := range ids {
		cleanupFixtureChat(t, a, id, "codex", ws, "window-"+id)
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.WorkspaceRecords = []WorkspaceRecord{record}
		s.SessionTitles["claude:head"] = "Current title"
		s.SessionProviders["claude:head"] = "claude"
		s.ChatBindings = map[string]string{"codex:old": "claude:head"}
		s.BusSessions["codex:old"] = s.BusSessions["claude:head"]
		s.RemovedChats = map[string]chatRemoval{"codex:bin": {Stage: "future-reserved-stage"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.busHost().With(func(b *bus.Bus) error {
		ctx := context.Background()
		if _, err := b.SetLead(ctx, "window-codex:fork", "window-claude:head"); err != nil {
			return err
		}
		lead, err := b.Session(ctx, "window-claude:head")
		if err != nil {
			return err
		}
		member, err := b.Session(ctx, "window-codex:fork")
		if err != nil {
			return err
		}
		if _, err := b.CreateGroup(ctx, bus.Sender{}, "Invented review group", []string{lead.Address(), member.Address()}); err != nil {
			return err
		}
		for _, id := range ids {
			if err := b.EndSession(ctx, "window-"+id); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestOrgChartUsesCanonicalSnapshotWithoutProviderDiscovery(t *testing.T) {
	starts := orgChartForbidProviderStarts(t)
	a, connection := providerTestApp(t)
	ws := t.TempDir()
	seedOrgChartFixture(t, a, ws)
	before := len(connection.calls.Snapshot())
	view, err := a.OrgChart()
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Chats) != 2 || len(view.Teams) != 1 || view.Teams[0].LeadID != "codex:old" || len(view.Teams[0].MemberIDs) != 1 || view.Teams[0].MemberIDs[0] != "codex:fork" {
		t.Fatalf("canonical inventory/edges changed: %+v", view)
	}
	if len(view.Groups) == 0 {
		t.Fatal("ordinary local group is missing")
	}
	if len(connection.calls.Snapshot()) != before || starts.Load() != 0 {
		t.Fatal("chart queried or started a provider")
	}
	for _, chat := range view.Chats {
		if chat.Resolution != "resolved" || chat.Workspace != ws || chat.State.Life != lifeClosed {
			t.Fatalf("bad local metadata: %+v", chat)
		}
	}
	again, err := a.OrgChart()
	if err != nil || again.Generation != view.Generation {
		t.Fatalf("unchanged snapshot changed generation: %v", err)
	}
	if err := a.updateSettings(func(s *Settings) error { s.SessionTitles["claude:head"] = "Changed title"; return nil }); err != nil {
		t.Fatal(err)
	}
	again, err = a.OrgChart()
	if err != nil || again.Generation == view.Generation {
		t.Fatalf("changed snapshot kept generation: %v", err)
	}
}

func TestOrgChartKeepsRepairOwnershipExplicit(t *testing.T) {
	starts := orgChartForbidProviderStarts(t)
	a, connection := providerTestApp(t)
	a.settings.SessionTitles = map[string]string{"root": "Needs repair"}
	a.settings.ChatBindings = map[string]string{"root": "first"}
	a.settings.ChatTransferHistory = []chatTransfer{{Source: "root", Target: "second", Stage: "committed"}}
	view, err := a.OrgChart()
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Chats) == 0 {
		t.Fatal("ambiguous local ownership disappeared")
	}
	for _, chat := range view.Chats {
		if chat.Resolution != "repair" || chat.Reason == "" || chat.Head != "" {
			t.Fatalf("repair ownership guessed: %+v", chat)
		}
	}
	if len(view.Teams) != 0 || len(connection.calls.Snapshot()) != 0 || starts.Load() != 0 {
		t.Fatal("repair query invented an edge or contacted a provider")
	}
}
