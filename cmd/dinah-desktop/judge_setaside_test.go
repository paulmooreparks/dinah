package main

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"studio/internal/judge"
	"studio/internal/ollamaagent"
)

func offeredTurns() *ollamaagent.AsideTurns {
	return &ollamaagent.AsideTurns{Needed: 1500, Turns: []ollamaagent.AsideTurn{
		{N: 1, Said: "Hi, can you help?", Answered: "Of course.", Cost: 800},
		{N: 2, Said: "Decision: keep the API stable.", Answered: "Understood.", Cost: 700},
		{N: 3, Said: "Thanks.", Answered: "Any time.", Cost: 900},
		{N: 4, Said: "Nice weather.", Answered: "Indeed.", Cost: 800},
		{N: 5, Said: "Add the writer.", Answered: "Done.", Cost: 600},
	}}
}

// setAsideApp opens a chat on a test connection that offers turns, with a model
// chosen and the kind on or off.
func setAsideApp(t *testing.T, on bool) (*App, *testProviderConnection, *Session) {
	t.Helper()
	a, _ := calibrationApp(t)
	c := a.runtime.ProviderLocked("codex").Connection().(*testProviderConnection)
	withContextModule(t, "codex")
	c.offered = offeredTurns()
	choose(t, a, func(s *JudgementSettings) { s.Model = "tiny:4b" })
	if on {
		choose(t, a, func(s *JudgementSettings) {
			s.KindsOff = slices.DeleteFunc(slices.Clone(s.KindsOff), func(n string) bool { return n == "set-aside-choice" })
		})
	}
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return a, c, a.session(info.ID)
}

func asideRequestTurns(t *testing.T, req any) []int {
	t.Helper()
	m, ok := req.(map[string]any)
	if !ok {
		return nil
	}
	turns, _ := m["turns"].([]int)
	return turns
}

// The kind that shows a judge a line from each of many turns starts off, and with
// it off nothing is sent to the judge and the oldest turns are set aside as before.
func TestSetAsideChoiceStartsOffAndThenTheAdaptersRuleStands(t *testing.T) {
	a, c, s := setAsideApp(t, false)
	k := kindNamed(t, a, "set-aside-choice")
	if k.Enabled || !k.DefaultOff || k.InputCap != 12000 || k.DeadlineMs != 10000 {
		t.Fatalf("the kind %+v", k)
	}
	c.asideTurns = 2
	n, err := ollamaSetAside(a, a.sessionConn(s), s.ID())
	if err != nil || n != 2 || len(c.asideRequests) != 1 || asideRequestTurns(t, c.asideRequests[0]) != nil {
		t.Fatalf("turns %d, requests %v, %v", n, c.asideRequests, err)
	}
	if rows, _ := a.JudgementRecords(JudgementFilter{Kind: "set-aside-choice"}); len(rows.Records) != 1 || rows.Records[0].Why != "off" {
		t.Fatalf("records %+v", rows.Records)
	}
}

// With the kind on, the judge is shown the turns offered and what must be freed,
// and the turns it chooses are what the adapter is asked to set aside, in place of
// the oldest.
func TestTheTurnsAJudgeChoosesAreWhatTheAdapterIsAsked(t *testing.T) {
	a, c, s := setAsideApp(t, true)
	c.asideTurns = 2
	n, err := ollamaSetAside(a, a.sessionConn(s), s.ID())
	if err != nil || n != 2 || len(c.asideRequests) != 1 {
		t.Fatalf("turns %d, requests %v, %v", n, c.asideRequests, err)
	}
	// The stand-in skips the first two turns and takes 3 and 4, which free 1700.
	if got := asideRequestTurns(t, c.asideRequests[0]); !slices.Equal(got, []int{3, 4}) {
		t.Fatalf("the adapter was asked to set aside %v: %v", got, c.asideRequests[0])
	}
	rows, _ := a.JudgementRecords(JudgementFilter{Kind: "set-aside-choice"})
	if len(rows.Records) != 1 || rows.Records[0].Used != "judge" || rows.Records[0].EffectKind != "chat" || rows.Records[0].EffectID != s.ID() {
		t.Fatalf("records %+v", rows.Records)
	}
	var in setAsideIn
	if err := json.Unmarshal([]byte(rows.Records[0].Input), &in); err != nil || in.Needed != 1500 || len(in.Turns) != 5 || in.Turns[1].Said != "Decision: keep the API stable." || in.Turns[1].Cost != 700 {
		t.Fatalf("the judge was shown %s", rows.Records[0].Input)
	}
}

// An answer that is not allowed by the measured costs is replaced by the rule, and
// so is a list the adapter refuses; and a call that could not free what is needed
// from the turns that fit is not made.
func TestAnAnswerThatIsNotAllowedFallsBackToTheOldestTurns(t *testing.T) {
	a, c, s := setAsideApp(t, true)
	set, _ := a.judges()
	for name, answer := range map[string]string{
		"a turn that was not offered": `{"turns":[99]}`,
		"too little":                  `{"turns":[1]}`,
		"a turn twice":                `{"turns":[3,3,4]}`,
		"no turn":                     `{"turns":[]}`,
	} {
		set.SetJudge(&answering{answer}, "test", "tokens")
		c.asideRequests = nil
		if n, err := ollamaSetAside(a, a.sessionConn(s), s.ID()); err != nil || len(c.asideRequests) != 1 || asideRequestTurns(t, c.asideRequests[0]) != nil || n != 0 {
			t.Errorf("%s: turns %d, requests %v, %v", name, n, c.asideRequests, err)
		}
	}
	// The adapter refuses a list a judge's answer passed, and the oldest are set aside.
	set.SetJudge(&answering{`{"turns":[3,4]}`}, "test", "tokens")
	c.asideRequests, c.refuseChosen, c.asideTurns = nil, true, 4
	n, err := ollamaSetAside(a, a.sessionConn(s), s.ID())
	if err != nil || n != 4 || len(c.asideRequests) != 2 || asideRequestTurns(t, c.asideRequests[0]) == nil || asideRequestTurns(t, c.asideRequests[1]) != nil {
		t.Fatalf("turns %d, requests %v, %v", n, c.asideRequests, err)
	}
	// Turns that cannot free what is needed even all together are never shown.
	c.refuseChosen, c.asideRequests = false, nil
	c.offered = &ollamaagent.AsideTurns{Needed: 99999, Turns: offeredTurns().Turns}
	before, _ := a.JudgementRecords(JudgementFilter{Kind: "set-aside-choice"})
	if _, err := ollamaSetAside(a, a.sessionConn(s), s.ID()); err != nil {
		t.Fatal(err)
	}
	if after, _ := a.JudgementRecords(JudgementFilter{Kind: "set-aside-choice"}); len(after.Records) != len(before.Records) {
		t.Fatal("the judge was asked about turns that cannot free enough")
	}
}

// The check is the measured costs: every turn offered, none twice, enough freed.
func TestTheSetAsideCheckIsTheMeasuredCosts(t *testing.T) {
	in := setAsideIn{Needed: 1000, Turns: []setAsideTurn{{N: 1, Cost: 600}, {N: 2, Cost: 500}, {N: 3, Cost: 400}}}
	for _, ok := range [][]int{{1, 2}, {1, 3}, {2, 1}, {1, 2, 3}} {
		if err := checkSetAside(in, setAsideOut{Turns: ok}); err != nil {
			t.Errorf("%v was refused: %v", ok, err)
		}
	}
	for _, bad := range [][]int{nil, {1}, {2, 3}, {1, 1, 2}, {4}, {0, 1, 2}, {-1}} {
		if err := checkSetAside(in, setAsideOut{Turns: bad}); err == nil {
			t.Errorf("%v was accepted", bad)
		}
	}
	// The rule is the oldest turns until enough is freed.
	if got := setAsideKind.Rule(in); !reflect.DeepEqual(got, setAsideOut{Turns: []int{1, 2}}) {
		t.Fatalf("the rule chose %v", got)
	}
	var _ = judge.UsedJudge
	var _ = context.Background
}
