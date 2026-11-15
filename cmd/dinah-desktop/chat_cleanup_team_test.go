package main

import (
	"context"
	"os"
	"reflect"
	"slices"
	"testing"

	"studio/internal/bus"
)

func TestCleanupCurrentLeadRefusedByEveryEntry(t *testing.T) {
	a, peer, _ := orgActionsFixture(t)
	removed := 0
	a.sink = func(name string, _ any) {
		if name == "chat:removed" {
			removed++
		}
	}
	stale := orgAction(t, a, "codex:a", "delete_chat")
	made := applyOrg(t, a, OrgChartActionRequest{Kind: "create_team", LeadID: "codex:a", Title: "Invented protected team", MemberIDs: []string{"codex:b"}})
	if made.Status != "succeeded" {
		t.Fatal(made)
	}
	reason := bus.TeamLeadAssignmentReason("Invented protected team")
	before, err := os.ReadFile(transcriptPath("codex:a"))
	if err != nil {
		t.Fatal(err)
	}
	for _, remove := range []func(string) error{a.RemoveChat, a.DeleteSession, a.HideSession} {
		if err := remove("codex:a"); err == nil || err.Error() != reason {
			t.Fatalf("lead deletion was not refused: %v", err)
		}
	}
	listing, err := a.ListChatCleanup()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(listing.Rows, func(row ChatCleanupRow) bool {
		return row.SessionID == "codex:a" && !row.Eligible && row.Refusal == reason
	}) {
		t.Fatal("cleanup offers current lead", listing)
	}
	view, err := a.OrgChartActions("codex:a")
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(view.Actions, func(action OrgChartAction) bool { return action.Kind == "delete_chat" }) {
		t.Fatal("chart offers current lead deletion")
	}
	result, err := a.ApplyOrgChartAction(OrgChartActionRequest{Kind: "delete_chat", ChatID: "codex:a", Token: stale.Token})
	if err != nil || result.Status != "refused" || result.Message != reason {
		t.Fatal("stale delete authority was accepted", result, err)
	}
	after, err := os.ReadFile(transcriptPath("codex:a"))
	if err != nil || string(before) != string(after) || removed != 0 || len(peer.calls.Snapshot()) != 0 || a.removedChat("codex:a") {
		t.Fatal("refusal changed chat or provider state", err, removed, peer.calls)
	}
}

func TestCleanupBulkProtectsLeadAndProcessesOtherRows(t *testing.T) {
	a, _, _ := orgActionsFixture(t)
	made := applyOrg(t, a, OrgChartActionRequest{Kind: "create_team", LeadID: "codex:a", Title: "Invented bulk team", MemberIDs: []string{"codex:b"}})
	if made.Status != "succeeded" {
		t.Fatal(made)
	}
	var before []bus.TeamMember
	if err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		before, err = b.TeamMembers(context.Background(), "window-codex:a")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	results := a.RemoveChats([]string{"codex:a", "codex:c", "codex:d"})
	if len(results) != 3 || results[0].OK || !results[1].OK || !results[2].OK {
		t.Fatal(results)
	}
	if results[0].Error != bus.TeamLeadAssignmentReason("Invented bulk team") {
		t.Fatal(results[0])
	}
	if a.removedChat("codex:a") || !a.removedChat("codex:c") || !a.removedChat("codex:d") {
		t.Fatal("bulk removal did not remain per-row")
	}
	if err := a.busHost().With(func(b *bus.Bus) error {
		after, err := b.TeamMembers(context.Background(), "window-codex:a")
		if !reflect.DeepEqual(before, after) {
			t.Error("bulk removal changed reporting rows", after)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupFormerLeadCanBeDeletedAfterEitherExit(t *testing.T) {
	for _, mode := range []string{"reassign", "replace"} {
		t.Run(mode, func(t *testing.T) {
			a, _, _ := orgActionsFixture(t)
			made := applyOrg(t, a, OrgChartActionRequest{Kind: "create_team", LeadID: "codex:a", Title: "Invented retained channel", MemberIDs: []string{"codex:b"}})
			if made.Status != "succeeded" {
				t.Fatal(made)
			}
			if err := a.RemoveChat("codex:a"); err == nil {
				t.Fatal("lead was deletable before exit")
			}
			if mode == "reassign" {
				if _, err := a.SetTeamLead("codex:b", "codex:c"); err != nil {
					t.Fatal(err)
				}
			} else if result := applyOrg(t, a, OrgChartActionRequest{Kind: "replace_team_lead", ChatID: "codex:b", GroupID: made.GroupID}); result.Status != "succeeded" {
				t.Fatal(result)
			}
			if err := a.DeleteSession("codex:a"); err != nil {
				t.Fatal("former lead could not be deleted", err)
			}
			if !a.removedChat("codex:a") {
				t.Fatal("former lead did not enter bin")
			}
			if room, err := a.BusGroup(made.GroupID); err != nil || room.Name != made.GroupID {
				t.Fatal("former team channel was destroyed", room, err)
			}
		})
	}
}
