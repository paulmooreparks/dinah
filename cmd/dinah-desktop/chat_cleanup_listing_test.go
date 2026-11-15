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
	before := len(c.calls)
	listing, err := a.ListChatCleanup()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.calls) != before {
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
