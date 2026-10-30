package judge

import (
	"path/filepath"
	"testing"
	"time"
)

func addCall(t *testing.T, s *Store, used, why, answer string) int64 {
	t.Helper()
	id, err := s.Add(Record{Time: time.Now(), Kind: "chat-title", Version: "v", Used: used, Why: why, Answer: answer, Provider: "p", Input: "{}", RuleAnswer: `""`, Out: `""`})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// Warnings are the undismissed fallbacks of the current streak; dismissing one, or all, keeps them out of the count for
// good, a fallback after a dismissal is a new warning, and a restart keeps the dismissals.
func TestWarningsAreDismissedForGoodAndAFreshFallbackIsNew(t *testing.T) {
	path := filepath.Join(t.TempDir(), "judgements.db")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	addCall(t, s, UsedJudge, "", "fine")
	a := addCall(t, s, UsedRule, WhyError, "ollama: quota used up (ref: 1)")
	b := addCall(t, s, UsedRule, WhyLate, "")
	addCall(t, s, UsedRule, WhyOff, "")    // the person's own switch: not a warning
	addCall(t, s, UsedRule, WhyPaused, "") // a consequence of the pause: not a warning of its own
	c := addCall(t, s, UsedRule, WhyForm, "")
	ws, err := s.Warnings()
	if err != nil || len(ws) != 3 || ws[0].ID != c || ws[1].ID != b || ws[2].ID != a || ws[2].Reason != "ollama: quota used up" {
		t.Fatalf("%+v %v", ws, err)
	}
	if ok, err := s.DismissWarning(b); err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
	if ws, _ = s.Warnings(); len(ws) != 2 || ws[0].ID != c || ws[1].ID != a {
		t.Fatalf("after dismissing one: %+v", ws)
	}
	if ok, _ := s.DismissWarning(9999); ok {
		t.Fatal("a call that does not exist was dismissed")
	}
	n, err := s.DismissWarnings()
	if err != nil || n != 2 {
		t.Fatalf("dismissed %d: %v", n, err)
	}
	if ws, _ = s.Warnings(); len(ws) != 0 {
		t.Fatalf("after dismissing all: %+v", ws)
	}
	// It stays cleared across a restart.
	s.Close()
	if s, err = OpenStore(path); err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if ws, _ = s.Warnings(); len(ws) != 0 {
		t.Fatalf("after a restart: %+v", ws)
	}
	// A new fallback is a new warning, and an answer from the judge ends the streak.
	d := addCall(t, s, UsedRule, WhyError, "x")
	if ws, _ = s.Warnings(); len(ws) != 1 || ws[0].ID != d {
		t.Fatalf("a fresh fallback: %+v", ws)
	}
	addCall(t, s, UsedJudge, "", "ok")
	if ws, _ = s.Warnings(); len(ws) != 0 {
		t.Fatalf("after the judge answered: %+v", ws)
	}
}
