package main

import (
	"context"
	"slices"
	"testing"

	"studio/internal/bus"
	"studio/internal/sessionhost"
)

func TestStaleAddressesAreThoseOfEarlierStagesAndDeletedChats(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	// A chat switched once in an older build: each stage has its own address.
	earlierStageFixture(t, a, "claude:first", "claude", ws, "window-first", "")
	earlierStageFixture(t, a, "codex:second", "codex", ws, "window-second", "claude:first")
	// A chat an older build deleted, which kept its address.
	earlierStageFixture(t, a, "ollama:deleted", "ollama", ws, "window-deleted", "")
	if err := writeTranscript(&sessionhost.Transcript{SessionID: "ollama:deleted", Provider: "ollama", Workspace: ws, Complete: true, Deleted: true}); err != nil {
		t.Fatal(err)
	}
	// A hidden chat that still has its saved conversation, and a chat that
	// was never switched: both keep their addresses.
	earlierStageFixture(t, a, "codex:hidden", "codex", ws, "window-hidden", "")
	earlierStageFixture(t, a, "claude:alone", "claude", ws, "window-alone", "")
	if err := a.updateSettings(func(s *Settings) error { s.Hidden = []string{"codex:hidden"}; return nil }); err != nil {
		t.Fatal(err)
	}
	stale, err := a.staleAddresses()
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, address := range stale {
		got = append(got, address.Recipient)
	}
	if !slices.Equal(got, []string{"window-deleted", "window-first"}) && !slices.Equal(got, []string{"window-first", "window-deleted"}) {
		t.Fatalf("the stale addresses are %v", got)
	}
	if err := a.retireStaleAddresses(); err != nil {
		t.Fatal(err)
	}
	available := func(recipient string) bool {
		var yes bool
		if err := a.busHost().With(func(b *bus.Bus) error {
			var err error
			yes, err = b.RecipientAvailable(context.Background(), recipient)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return yes
	}
	for recipient, want := range map[string]bool{"window-first": false, "window-deleted": false, "window-second": true, "window-hidden": true, "window-alone": true} {
		if available(recipient) != want {
			t.Fatalf("%s is available: %v, want %v", recipient, !want, want)
		}
	}
	if again, err := a.staleAddresses(); err != nil || len(again) != 0 {
		t.Fatalf("a second pass finds %v, %v", again, err)
	}
	// Retiring can be undone.
	if err := a.busHost().With(func(b *bus.Bus) error { return b.RestoreRecipient(context.Background(), "window-first") }); err != nil {
		t.Fatal(err)
	}
	if !available("window-first") {
		t.Fatal("a retired address could not be brought back")
	}
}

func TestHiddenChatsGoToTheRecycleBin(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	// A chat an older build hid when the person asked to delete it, with an
	// earlier stage; a listed chat with the hidden target of a switch that did
	// not finish; and a listed chat left marked hidden, which is no hidden chat.
	earlierStageFixture(t, a, "claude:hidden-first", "claude", ws, "", "")
	earlierStageFixture(t, a, "codex:hidden-newest", "codex", ws, "window-hidden", "claude:hidden-first")
	earlierStageFixture(t, a, "claude:listed", "claude", ws, "window-listed", "")
	if err := writeTranscript(&sessionhost.Transcript{SessionID: "codex:unfinished", Provider: "codex", Workspace: ws, ParentSessionID: "claude:listed", Complete: true, PendingHandoff: true}); err != nil {
		t.Fatal(err)
	}
	earlierStageFixture(t, a, "claude:marked-first", "claude", ws, "", "")
	earlierStageFixture(t, a, "codex:marked-newest", "codex", ws, "window-marked", "claude:marked-first")
	if err := a.updateSettings(func(s *Settings) error {
		s.Hidden = []string{"claude:hidden-first", "codex:hidden-newest", "codex:unfinished", "codex:marked-newest"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ids, err := a.hiddenChatsToRemove()
	if err != nil || !slices.Equal(ids, []string{"codex:hidden-newest"}) {
		t.Fatalf("the hidden chats are %v, %v", ids, err)
	}
	if err := a.removeHiddenChats(); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"claude:hidden-first", "codex:hidden-newest"} {
		if record, ok := a.cleanupRecord(id); !ok || record.Stage != "retained" || record.Group != "codex:hidden-newest" {
			t.Fatalf("%s has the removal record %+v", id, record)
		}
	}
	for _, id := range []string{"claude:listed", "codex:unfinished", "claude:marked-first", "codex:marked-newest"} {
		if _, reserved := a.cleanupRecord(id); reserved {
			t.Fatalf("%s was removed", id)
		}
	}
	if restored := a.RestoreRemovedChat("codex:hidden-newest"); !restored.OK {
		t.Fatalf("a hidden chat in the Recycle Bin could not be restored: %+v", restored)
	}
}
