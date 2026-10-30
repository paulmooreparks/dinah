package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func costUpdate(t *testing.T, a *App, chat string, tokens int) {
	t.Helper()
	params, _ := json.Marshal(map[string]any{"sessionId": chat, "update": map[string]any{"sessionUpdate": "turn_cost", "tokens": tokens}})
	a.newAgentHandler().Notify("session/update", params)
}

func budgetRow(t *testing.T, a *App, lead *Session, provider string) TeamBudgetRow {
	t.Helper()
	view, err := a.Team(lead.ID())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range view.Budget.Rows {
		if r.Provider == provider {
			return r
		}
	}
	t.Fatalf("no row for %s in %+v", provider, view.Budget)
	return TeamBudgetRow{}
}

// What a member's turns used is counted per provider from the adapters' reports; a turn that reported nothing is unknown
// and makes the figure incomplete, with no remaining shown; passing the allowance warns once and holds nothing; the page is
// not shown the reports.
func TestATeamsBudgetCountsReportedUsageAndSaysWhenItIsIncomplete(t *testing.T) {
	a, lead, member, _, _, _ := teamApp(t)
	var events []string
	a.sink = func(name string, _ any) { events = append(events, name) }
	row, err := a.SetTeamBudget(lead.ID(), []TeamAllowanceInput{{Provider: "codex", Allowance: 2000}})
	if err != nil || len(row.Rows) != 1 || row.Rows[0].Unit != "tokens" || row.Rows[0].Allowance != 2000 || !row.Rows[0].Complete || row.Rows[0].Remaining == nil || *row.Rows[0].Remaining != 2000 {
		t.Fatalf("the budget as set: %+v %v", row, err)
	}
	costUpdate(t, a, member.ID(), 700)
	costUpdate(t, a, member.ID(), 300)
	a.recordTurnUsage(member)
	r := budgetRow(t, a, lead, "codex")
	if r.Used != 1000 || r.Unknown != 0 || !r.Complete || r.Remaining == nil || *r.Remaining != 1000 || r.Over || r.Today != 1000 {
		t.Fatalf("after a counted turn: %+v", r)
	}
	// A turn that reported nothing is unknown, and the figure is no longer complete.
	a.recordTurnUsage(member)
	r = budgetRow(t, a, lead, "codex")
	if r.Unknown != 1 || r.Complete || r.Remaining != nil || r.Used != 1000 || r.TodayGaps != 1 {
		t.Fatalf("after an unknown turn: %+v", r)
	}
	// Passing the allowance is over, whatever the uncounted turns came to, and warns once.
	costUpdate(t, a, member.ID(), 1500)
	a.recordTurnUsage(member)
	if r = budgetRow(t, a, lead, "codex"); !r.Over || r.Used != 2500 {
		t.Fatalf("past the allowance: %+v", r)
	}
	view, _ := a.Team(lead.ID())
	a.warnOverAllowances(view, "Lead")
	a.warnOverAllowances(view, "Lead")
	var cards int
	items, _ := a.Notifications()
	for _, n := range items {
		if strings.HasPrefix(n.ID, "team-allowance-") {
			cards++
		}
	}
	if cards != 1 {
		t.Fatalf("%d warnings", cards)
	}
	// The reports are not forwarded to the page.
	for _, e := range events {
		if e == "agent:update" {
			t.Fatalf("a turn_cost report was shown: %v", events)
		}
	}
	// A chat that is not a team member has its turns recorded nowhere.
	if _, err := a.SetTeamBudget(lead.ID(), []TeamAllowanceInput{{Provider: "nothing", Allowance: 1}}); err == nil {
		t.Fatal("an allowance for an unknown provider was accepted")
	}
	if _, err := a.SetTeamBudget(lead.ID(), []TeamAllowanceInput{{Provider: "codex", Allowance: 1}, {Provider: "codex", Allowance: 2}}); err == nil {
		t.Fatal("two allowances for one provider were accepted")
	}
	costUpdate(t, a, lead.ID(), 999)
	a.recordTurnUsage(lead)
	if r = budgetRow(t, a, lead, "codex"); r.Used != 2500 {
		t.Fatalf("a chat with no lead was counted: %+v", r)
	}
	if cleared, err := a.SetTeamBudget(lead.ID(), nil); err != nil || len(cleared.Rows) == 0 {
		t.Fatalf("clearing: %+v %v", cleared, err)
	}
}
