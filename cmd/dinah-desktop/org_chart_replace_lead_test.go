package main

import (
	"context"
	"slices"

	"testing"

	"studio/internal/bus"
)

func TestOrgChartReplaceLeadRetainsTeamAndRefusesLeadAssignment(t *testing.T) {
	a, peer, _ := orgActionsFixture(t)
	created := applyOrg(t, a, OrgChartActionRequest{Kind: "create_team", LeadID: "codex:a", Title: "Invented stable team", MemberIDs: []string{"codex:b", "codex:d"}})
	if created.Status != "succeeded" {
		t.Fatal(created)
	}
	view, err := a.OrgChartActions("codex:a")
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(view.Actions, func(v OrgChartAction) bool { return v.Kind == "assign_team" }) {
		t.Fatal("current lead can be assigned", view)
	}
	if !slices.ContainsFunc(view.Unavailable, func(v OrgChartUnavailable) bool {
		return v.Kind == "assign_team" && v.Reason == bus.TeamLeadAssignmentReason("Invented stable team")
	}) {
		t.Fatal("lead refusal omitted ways out", view)
	}
	if _, err := a.SetTeamLead("codex:a", "codex:c"); err == nil || err.Error() != bus.TeamLeadAssignmentReason("Invented stable team") {
		t.Fatal("direct host assignment accepted lead", err)
	}
	action := orgAction(t, a, "codex:b", "replace_team_lead")
	beforeChart, err := a.OrgChart()
	if err != nil || len(beforeChart.Teams) != 1 {
		t.Fatal(beforeChart, err)
	}
	if len(action.Groups) != 1 || action.Groups[0].ID != created.GroupID {
		t.Fatal("replacement did not offer stable team group", action)
	}
	for _, bad := range []OrgChartActionRequest{
		{Kind: "replace_team_lead", ChatID: "codex:b", GroupID: "invented-wrong-group"},
		{Kind: "replace_team_lead", ChatID: "codex:b", GroupID: created.GroupID, MemberIDs: []string{"codex:c"}},
		{Kind: "replace_team_lead", ChatID: "codex:b", GroupID: created.GroupID, LeadID: "codex:c"},
	} {
		if result := applyOrg(t, a, bad); result.Status != "refused" {
			t.Fatal("forged request accepted", result)
		}
	}
	result := applyOrg(t, a, OrgChartActionRequest{Kind: "replace_team_lead", ChatID: "codex:b", GroupID: created.GroupID})
	if result.Status != "succeeded" || result.GroupID != created.GroupID || result.Message == "" || len(result.Recovery) != 1 {
		t.Fatal(result)
	}
	afterChart, err := a.OrgChart()
	if err != nil || len(afterChart.Teams) != 1 || beforeChart.Teams[0].ID != afterChart.Teams[0].ID || beforeChart.Teams[0].Title != afterChart.Teams[0].Title {
		t.Fatal("replacement changed team identity or title", beforeChart, afterChart, err)
	}
	if err := a.busHost().With(func(b *bus.Bus) error {
		if parent, _, err := b.LeadOf(context.Background(), "window-codex:a"); err != nil || parent != "window-codex:b" {
			t.Fatalf("old lead did not become member: %s %v", parent, err)
		}
		if group, err := b.TeamGroup(context.Background(), "window-codex:b"); err != nil || group != created.GroupID {
			t.Fatalf("team chat changed: %s %v", group, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetTeamLead("codex:a", "codex:c"); err != nil {
		t.Fatal("former lead cannot move after replacement", err)
	}
	if len(peer.calls) != 0 {
		t.Fatal("team management contacted provider")
	}
}

func TestOrgChartReplaceLeadRejectsStaleMembership(t *testing.T) {
	a, _, _ := orgActionsFixture(t)
	created := applyOrg(t, a, OrgChartActionRequest{Kind: "create_team", LeadID: "codex:a", Title: "Invented stale team", MemberIDs: []string{"codex:b", "codex:d"}})
	action := orgAction(t, a, "codex:b", "replace_team_lead")
	if _, err := a.SetTeamLead("codex:b", "codex:c"); err != nil {
		t.Fatal(err)
	}
	result, err := a.ApplyOrgChartAction(OrgChartActionRequest{Kind: "replace_team_lead", ChatID: "codex:b", GroupID: created.GroupID, Token: action.Token})
	if err != nil || result.Status != "refused" || result.Message == "" {
		t.Fatal(result, err)
	}
}
