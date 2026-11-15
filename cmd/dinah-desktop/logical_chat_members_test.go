package main

import (
	"testing"
	"time"
)

func TestCleanupLogicalMembershipKeepsDistinctRecipientsSeparate(t *testing.T) {
	s := Settings{SessionProviders: map[string]string{"source": "codex", "target": "claude"}, BusSessions: map[string]string{"source": "window-one", "target": "window-two"}, ChatTransfers: map[string]chatTransfer{"source": {Source: "source", Target: "target", Recipient: "window-one", Stage: "prepared", Revision: "attempt"}}}
	members, err := logicalChatMembersFromSettings(s, "source")
	if err != nil || len(members.Members) != 1 || members.Members[0].SessionID != "source" || len(members.SeparateRecipients) != 1 {
		t.Fatalf("separate ownership %+v %v", members, err)
	}
	if target, err := logicalChatMembersFromSettings(s, "target"); err != nil || len(target.Members) != 1 || target.Head != "target" {
		t.Fatalf("target ownership %+v %v", target, err)
	}
}

func TestCleanupLogicalMembershipUnknownAllocationAndGeneration(t *testing.T) {
	s := Settings{ChatTransfers: map[string]chatTransfer{"source": {Source: "source", Provider: "claude", Stage: "allocating", Revision: "attempt", Created: time.Unix(10, 0)}}}
	members, err := logicalChatMembersFromSettings(s, "source")
	if err != nil || len(members.Members) != 1 || len(members.UnresolvedAllocations) != 1 || members.UnresolvedAllocations[0].AttemptID != "attempt" {
		t.Fatalf("allocation %+v %v", members, err)
	}
	s.ChatTransfers["source"] = chatTransfer{Source: "source", Target: "allocated", Provider: "claude", Stage: "allocated", Revision: "attempt"}
	changed, err := logicalChatMembersFromSettings(s, "source")
	if err != nil || changed.Generation == members.Generation || len(changed.Members) != 2 {
		t.Fatalf("generation %+v %v", changed, err)
	}
	for i := 0; i < 20; i++ {
		again, err := logicalChatMembersFromSettings(s, "source")
		if err != nil || again.Generation != changed.Generation {
			t.Fatal("unstable membership generation", err)
		}
	}
}

func TestCleanupLogicalMembershipRejectsAmbiguousAndCyclicOwnership(t *testing.T) {
	for _, s := range []Settings{
		{ChatBindings: map[string]string{"a": "b", "b": "a"}},
		{ChatBindings: map[string]string{"a": "b"}, ChatTransferHistory: []chatTransfer{{Source: "a", Target: "other", Stage: "committed"}}},
	} {
		if _, err := logicalChatMembersFromSettings(s, "a"); err == nil {
			t.Fatal("ambiguous ownership accepted")
		}
	}
}
