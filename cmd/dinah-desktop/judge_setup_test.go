package main

import (
	"testing"
)

// The first-start question: nothing is chosen until the person answers, Not
// Now keeps the question, Turn On chooses the provider with the starting
// budget, and an installation that already has a model is never asked.
func TestJudgementsAskOnceAndChooseNothingUntilAnswered(t *testing.T) {
	a, _, _ := judgementApp(t)
	st, err := a.JudgementState()
	if err != nil || st.State != "setup" || st.Setup == nil || st.Setup.Provider == "" || st.Setup.CallsPerDay != judgementStartCalls || st.Setup.Shown {
		t.Fatalf("before any answer: %+v %v", st, err)
	}
	if err := a.NoteJudgementSetupShown(); err != nil {
		t.Fatal(err)
	}
	if st, _ = a.JudgementState(); st.State != "setup" || !st.Setup.Shown {
		t.Fatalf("after the flyout opened by itself: %+v", st)
	}
	if err := a.DeferJudgementSetup(); err != nil {
		t.Fatal(err)
	}
	snap, _ := a.JudgementSettings()
	if st, _ = a.JudgementState(); st.State != "setup" || snap.Settings.Setup != "later" || snap.Settings.Model != "" {
		t.Fatalf("after Not Now: %+v %+v", st, snap.Settings)
	}
	// The page saves the settings without the answer, which is kept.
	page := snap.Settings
	page.Setup, page.SetupShown = "", false
	if _, err := a.SetJudgementSettings(page, snap.Revision); err != nil {
		t.Fatal(err)
	}
	if snap, _ = a.JudgementSettings(); snap.Settings.Setup != "later" || !snap.Settings.SetupShown {
		t.Fatalf("a save from the page lost the answer: %+v", snap.Settings)
	}
	if err := a.AcceptJudgementSetup(); err != nil {
		t.Fatal(err)
	}
	snap, _ = a.JudgementSettings()
	st, _ = a.JudgementState()
	if snap.Settings.Setup != "on" || snap.Settings.Provider != st.Provider || st.State != "off" || st.Why != "choosing a model" || st.Setup != nil {
		t.Fatalf("after Turn On: %+v %+v", st, snap.Settings)
	}
	// A model in the settings means judgements were set up, by whichever way.
	if s := normalizeJudgementSettings(JudgementSettings{Enabled: true, Model: "m"}); s.Setup != "on" {
		t.Fatalf("settings with a model: %+v", s)
	}
}

// The starting budget is the calls a day turned into the provider's unit for
// a week, where a call's cost is known, and none where the judge is local.
func TestTheStartingBudgetFollowsWhatACallCosts(t *testing.T) {
	a, _, _ := judgementApp(t)
	offer := a.judgementSetupOffer(JudgementSettings{})
	if offer.Provider == "" {
		t.Skip("no provider that can judge is offered in this fixture")
	}
	switch offer.Billing {
	case "local":
		if offer.Limit != nil {
			t.Fatalf("a local judge has a budget: %+v", offer)
		}
	default:
		if offer.Limit != nil && *offer.Limit%(judgementStartCalls*7) != 0 {
			t.Fatalf("the budget is not a week of calls: %+v", offer)
		}
	}
}
