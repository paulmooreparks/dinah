package main

import (
	"reflect"
	"testing"

	"studio/internal/sessionhost"
)

func TestCleanupListingReadsClosedChainsWithoutProviderCalls(t *testing.T) {
	a, c := providerTestApp(t)
	a.settings.SessionProviders = map[string]string{"old": "codex", "current": "claude", "unknown": "devin"}
	a.settings.SessionTitles = map[string]string{"current": "One logical chat", "unknown": "No saved transcript"}
	a.settings.ChatBindings = map[string]string{"old": "current"}
	a.settings.ChatTransfers = map[string]chatTransfer{"old": {Source: "old", Target: "current", Stage: "committed"}}
	for _, tr := range []*sessionhost.Transcript{
		{SessionID: "old", Provider: "codex", Workspace: "C:/invented/one", Messages: []ConversationMessage{{Role: "you", Text: "First"}, {Role: "agent", Text: "Answer"}, {Role: "tool", Text: "Tool"}}},
		{SessionID: "current", ParentSessionID: "old", Provider: "claude", Workspace: "C:/invented/one", Messages: []ConversationMessage{{Role: "you", Text: "Continue"}, {Role: "status", Text: "Idle"}}},
	} {
		if err := writeTranscript(tr); err != nil {
			t.Fatal(err)
		}
	}
	before := len(c.calls.Snapshot())
	listing, err := a.ListChatCleanup()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.calls.Snapshot()) != before {
		t.Fatal("listing contacted the provider")
	}
	if len(listing.Rows) != 2 {
		t.Fatalf("rows %+v", listing.Rows)
	}
	var current, unknown ChatCleanupRow
	for _, row := range listing.Rows {
		switch row.SessionID {
		case "current":
			current = row
		case "unknown":
			unknown = row
		}
	}
	if !reflect.DeepEqual(current.SessionIDs, []string{"old", "current"}) || current.MessageCount != 3 || !current.MessageCountKnown || current.Open {
		t.Fatalf("closed chain %+v", current)
	}
	if unknown.MessageCountKnown || unknown.MessageCount != 0 {
		t.Fatalf("unknown count %+v", unknown)
	}
	if len(listing.Removed) != 0 || !listing.ProviderDeletionEnabled {
		t.Fatalf("listing policy %+v", listing)
	}
}

// Removing one chat reads that chat's row alone. A full read counts the
// messages of every saved conversation, which on 16 November 2026 took long
// enough on one person's state that another chat changed during nearly every
// read, and each removal was refused.
func TestCleanupRowForOneChatLeavesTheOthersUnread(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	cleanupFixtureChat(t, a, "codex:wanted", "codex", ws, "window-wanted")
	cleanupFixtureChat(t, a, "claude:other", "claude", ws, "window-other")
	listing, err := a.cleanupRowFor("codex:wanted")
	if err != nil || len(listing.Rows) != 1 || listing.Rows[0].SessionID != "codex:wanted" || listing.Rows[0].MessageCount != 2 {
		t.Fatalf("the one row is %+v, %v", listing.Rows, err)
	}
	savedMessageCounts.Lock()
	_, counted := savedMessageCounts.files[transcriptPath("claude:other")]
	savedMessageCounts.Unlock()
	if counted {
		t.Fatal("the other chat's saved conversation was read")
	}
	if err := a.RemoveChat("codex:wanted"); err != nil {
		t.Fatal(err)
	}
	if listing, err = a.cleanupRowFor("codex:wanted"); err != nil || len(listing.Rows) != 0 || len(listing.Removed) != 1 {
		t.Fatalf("after removal the read gives %+v, %v", listing, err)
	}
}

func TestSavedMessageCountFollowsTheFile(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	cleanupFixtureChat(t, a, "codex:counted", "codex", ws, "")
	if count, known := savedMessageCount("codex:counted"); !known || count != 2 {
		t.Fatalf("counted %d, %v", count, known)
	}
	if err := writeTranscript(&sessionhost.Transcript{SessionID: "codex:counted", Provider: "codex", Workspace: ws, Complete: true, Messages: []ConversationMessage{{Role: "you", Text: "One"}, {Role: "agent", Text: "Two"}, {Role: "you", Text: "A third, longer message"}}}); err != nil {
		t.Fatal(err)
	}
	if count, known := savedMessageCount("codex:counted"); !known || count != 3 {
		t.Fatalf("after the file changed it counted %d, %v", count, known)
	}
	if _, known := savedMessageCount("codex:absent"); known {
		t.Fatal("a missing conversation has a known count")
	}
}
