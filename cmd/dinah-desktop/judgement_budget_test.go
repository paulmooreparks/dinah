package main

import (
	"context"
	"testing"

	"studio/internal/judge"
)

// A new install has no budget for any provider, a week that begins on Monday at
// midnight, and a pool of one to four; each provider's unit is read from its row.
func TestTheBudgetAndPoolStartAsDefaults(t *testing.T) {
	a, _, _ := judgementApp(t)
	snap, err := a.JudgementSettings()
	if err != nil {
		t.Fatal(err)
	}
	s := snap.Settings
	if len(s.Budgets) != len(judgeProviders()) || s.Budgets[0].Provider != judgeProviders()[0] || s.Budgets[0].Limit != nil || s.Budgets[0].Unit != "tokens" ||
		s.Period != (JudgementPeriod{Weekday: 1, Hour: 0}) || s.Pool != (JudgementPool{Min: 1, Max: 4}) {
		t.Fatalf("defaults %+v", s)
	}
	st, err := a.JudgementState()
	if err != nil || st.Budget == nil || len(st.Budget) != 0 || st.Pool.Size != 1 || st.Pool.Min != 1 || st.Pool.Max != 4 || st.Pool.GrowAt != judge.GrowFraction || st.Pool.ShrinkAfter != judge.ShrinkFactor {
		t.Fatalf("state %+v %v", st, err)
	}
	// With nothing judged the summary leaves its latest time out.
	if sum, _ := a.JudgementSummary("", ""); sum.Latest != nil {
		t.Fatalf("an empty summary has a latest time: %v", sum.Latest)
	}
}

// The budget, the period and the pool are the person's, checked before they are
// saved, kept with the other settings, and applied to the judges at once.
func TestTheBudgetPeriodAndPoolAreSavedCheckedAndApplied(t *testing.T) {
	a, events, _ := judgementApp(t)
	provider := judgeProviders()[0]
	limit := 5000
	saved := choose(t, a, func(s *JudgementSettings) {
		s.Budgets[0].Limit = &limit
		s.Period = JudgementPeriod{Weekday: 5, Hour: 9}
		s.Pool = JudgementPool{Min: 2, Max: 12}
	})
	if saved.Settings.Budgets[0].Limit == nil || *saved.Settings.Budgets[0].Limit != 5000 || saved.Settings.Pool.Max != 12 {
		t.Fatalf("saved %+v", saved.Settings)
	}
	st, _ := a.JudgementState()
	if len(st.Budget) != 1 || st.Budget[0].Provider != provider || st.Budget[0].Limit != 5000 || st.Budget[0].Unit != "tokens" || st.Budget[0].Left != 5000 ||
		st.Budget[0].PeriodStart.Weekday() != 5 || st.Budget[0].PeriodStart.Hour() != 9 || st.Pool.Min != 2 || st.Pool.Max != 12 || st.Pool.Size != 2 {
		t.Fatalf("state %+v", st)
	}
	if last := (*events)[len(*events)-1]; last["pool"] == nil || last["budget"] == nil {
		t.Fatalf("the event leaves out the budget or the pool: %v", last)
	}
	for name, edit := range map[string]func(*JudgementSettings){
		"negative limit":   func(s *JudgementSettings) { n := -1; s.Budgets[0].Limit = &n },
		"unknown":          func(s *JudgementSettings) { s.Budgets = append(s.Budgets, JudgementBudget{Provider: "nothing"}) },
		"weekday":          func(s *JudgementSettings) { s.Period.Weekday = 7 },
		"hour":             func(s *JudgementSettings) { s.Period.Hour = 24 },
		"pool minimum":     func(s *JudgementSettings) { s.Pool.Min = 0 },
		"pool bounds":      func(s *JudgementSettings) { s.Pool = JudgementPool{Min: 5, Max: 3} },
		"negative weekday": func(s *JudgementSettings) { s.Period.Weekday = -1 },
	} {
		bad := saved.Settings
		bad.Budgets = append([]JudgementBudget(nil), saved.Settings.Budgets...)
		edit(&bad)
		if _, err := a.SetJudgementSettings(bad, saved.Revision); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	if now, _ := a.JudgementSettings(); now.Revision != saved.Revision {
		t.Fatal("a refused save changed the settings")
	}
	// The limit can be cleared back to none, and a limit of 0 is a budget.
	choose(t, a, func(s *JudgementSettings) { zero := 0; s.Budgets[0].Limit = &zero })
	if st, _ := a.JudgementState(); len(st.Budget) != 1 || st.Budget[0].Limit != 0 {
		t.Fatalf("a budget of 0 %+v", st.Budget)
	}
	choose(t, a, func(s *JudgementSettings) { s.Budgets[0].Limit = nil })
	if st, _ := a.JudgementState(); len(st.Budget) != 0 {
		t.Fatalf("a cleared budget still has a row: %+v", st.Budget)
	}
	// A new window reads them from the file.
	second := NewApp()
	second.sink = func(string, any) {}
	second.judging.noAuto = true
	t.Cleanup(second.closeJudges)
	again, err := second.JudgementSettings()
	if err != nil || again.Settings.Period != (JudgementPeriod{Weekday: 5, Hour: 9}) || again.Settings.Pool != (JudgementPool{Min: 2, Max: 12}) {
		t.Fatalf("after a restart %+v %v", again, err)
	}
}

// A call refused for the budget is answered by its rule and the widget says so, with
// the provider; clearing the budget ends the warning.
func TestACallRefusedForTheBudgetIsAWarning(t *testing.T) {
	a, _, _ := judgementApp(t)
	choose(t, a, func(s *JudgementSettings) {
		s.Model = "tiny:4b"
		zero := 0
		s.Budgets[0].Limit = &zero
	})
	set, _ := a.judges()
	r := judge.Call(context.Background(), set, windowTestKind, windowTestIn{Line: "Rule line"}, judge.Effect{})
	if r.Used != judge.UsedRule || r.Why != judge.WhyBudget || r.Out != "Rule line" {
		t.Fatalf("the call %+v", r)
	}
	st, _ := a.JudgementState()
	if st.State != "warning" || st.Why != "the budget is spent for "+judgeProviders()[0] {
		t.Fatalf("state %+v", st)
	}
	rows, _ := a.JudgementRecords(JudgementFilter{Why: "budget"})
	if len(rows.Records) != 1 || rows.Records[0].Provider != judgeProviders()[0] {
		t.Fatalf("records %+v", rows.Records)
	}
	choose(t, a, func(s *JudgementSettings) { s.Budgets[0].Limit = nil })
	if r := judge.Call(context.Background(), set, windowTestKind, windowTestIn{Line: "Rule line"}, judge.Effect{}); r.Used != judge.UsedJudge {
		t.Fatalf("after the budget was cleared %+v", r)
	}
	if st, _ := a.JudgementState(); st.State != "on" {
		t.Fatalf("state %+v", st)
	}
}
