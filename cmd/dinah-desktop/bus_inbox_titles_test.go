package main

import (
	"context"
	"testing"

	"studio/internal/bus"
)

func TestInboxSenderTitlesFollowStableIdentity(t *testing.T) {
	a, _ := providerTestApp(t)
	ctx := context.Background()
	workspace := t.TempDir()
	var sender bus.Session
	var message bus.Message
	const body = "From yavchn-1: this historical preview stays unchanged"
	if err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		sender, err = b.RegisterSession(ctx, "stable-sender", workspace, "YAVCHN development")
		if err != nil {
			return err
		}
		message, err = b.Send(ctx, bus.Sender{Session: &sender}, bus.Human, body, nil, 0)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	check := func(want string) {
		t.Helper()
		exact, err := a.BusInboxMessage(message.ID)
		if err != nil || exact.From != want || exact.FromSession != sender.ID || exact.Text != body || !exact.Unread {
			t.Fatalf("exact item: %+v, want title %q, error %v", exact, want, err)
		}
		page, err := a.BusInboxPage("unread", 0)
		if err != nil || len(page.Items) != 1 || page.Items[0].From != want {
			t.Fatalf("page: %+v, error %v", page, err)
		}
		overview, err := a.BusOverview()
		if err != nil || len(overview.Inbox) != 1 || overview.Inbox[0].From != want {
			t.Fatalf("overview: %+v, error %v", overview.Inbox, err)
		}
	}
	check("YAVCHN development")
	if err := a.busHost().With(func(b *bus.Bus) error {
		if err := b.SetTitle(ctx, sender.ID, "Renamed development"); err != nil {
			return err
		}
		return b.EndSession(ctx, sender.ID)
	}); err != nil {
		t.Fatal(err)
	}
	check("Renamed development")
	if err := a.busHost().With(func(b *bus.Bus) error { return b.RemoveWorkspace(ctx, sender.WorkspaceName) }); err != nil {
		t.Fatal(err)
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.BusSessions = map[string]string{"native-history": sender.ID}
		s.SessionTitles = map[string]string{"native-history": "Saved historical title"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	check("Saved historical title")
	if err := a.updateSettings(func(s *Settings) error {
		s.BusSessions["different-owner"] = sender.ID
		s.SessionTitles["different-owner"] = "Conflicting owner"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	check("")
	if err := a.updateSettings(func(s *Settings) error {
		s.BusSessions = nil
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	check("")
}
