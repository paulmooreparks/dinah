package main

import (
	"slices"
	"strings"
	"testing"
)

func handoffLines(n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = "[Person] Decision " + strings.Repeat("d", 90) + " number " + string(rune('A'+i%26))
	}
	return lines
}

// turnOnHandoff is the person turning the kind on: leaving it out of the list of
// kinds that are off.
func turnOnHandoff(t *testing.T, a *App) {
	t.Helper()
	choose(t, a, func(s *JudgementSettings) {
		s.KindsOff = slices.DeleteFunc(slices.Clone(s.KindsOff), func(n string) bool { return n == "handoff-summary" })
	})
}

func kindNamed(t *testing.T, a *App, name string) JudgementKind {
	t.Helper()
	kinds, err := a.JudgementKinds()
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range kinds {
		if k.Name == name {
			return k
		}
	}
	t.Fatalf("the kind %s is not listed", name)
	return JudgementKind{}
}

// The kind that shows a judge much of a chat starts off, is listed as off with its
// cap and deadline, the person turns it on in the same list as every other, and
// that is kept.
func TestTheHandoffSummaryStartsOffAndThePersonTurnsItOn(t *testing.T) {
	a, _ := calibrationApp(t)
	k := kindNamed(t, a, "handoff-summary")
	if k.Enabled || !k.DefaultOff || k.InputCap != 12000 || k.DeadlineMs != 10000 || k.Correctable || len(k.Input) != 2 {
		t.Fatalf("the kind %+v", k)
	}
	snap, _ := a.JudgementSettings()
	if !slices.Contains(snap.Settings.KindsOff, "handoff-summary") {
		t.Fatalf("the page is not told the kind is off: %v", snap.Settings.KindsOff)
	}
	turnOnHandoff(t, a)
	if !kindNamed(t, a, "handoff-summary").Enabled {
		t.Fatal("the kind is still off after the person turned it on")
	}
	snap, _ = a.JudgementSettings()
	if slices.Contains(snap.Settings.KindsOff, "handoff-summary") || !slices.Contains(snap.Settings.KindsOn, "handoff-summary") {
		t.Fatalf("settings %+v", snap.Settings)
	}
	// A new window reads it from the file, and the person can turn it off again.
	second := NewApp()
	second.sink = func(string, any) {}
	second.judging.noAuto = true
	t.Cleanup(second.closeJudges)
	if !kindNamed(t, second, "handoff-summary").Enabled {
		t.Fatal("after a restart the kind is off")
	}
	choose(t, a, func(s *JudgementSettings) { s.KindsOff = append(s.KindsOff, "handoff-summary") })
	if kindNamed(t, a, "handoff-summary").Enabled {
		t.Fatal("the kind did not turn off")
	}
}

// With the kind off nothing is sent to the judge. Turned on, the judge is shown the
// lines and the room, within its cap, and the summary is kept for the same lines so
// that a preview and the continuation it leads to agree; a summary longer than its
// room, and an applet context in a line, never get through.
func TestTheHandoffDigestIsAskedOnlyWhenTheKindIsOnAndWithinItsCap(t *testing.T) {
	a, cs := calibrationApp(t)
	choose(t, a, func(s *JudgementSettings) { s.Model = "tiny:4b" })
	lines := handoffLines(6)
	if _, _, ok := a.judgeHandoffDigest("chat-1", lines, 5000); ok || cs.count("tiny:4b") != 0 {
		t.Fatalf("a judge was asked with the kind off: %v", cs.called)
	}
	turnOnHandoff(t, a)
	summary, shown, ok := a.judgeHandoffDigest("chat-1", lines, 5000)
	if !ok || shown != 6 || summary != lines[0] {
		t.Fatalf("summary %q shown %d ok %v", summary, shown, ok)
	}
	before := cs.count("tiny:4b")
	if again, n, ok := a.judgeHandoffDigest("chat-1", lines, 5000); !ok || n != 6 || again != summary || cs.count("tiny:4b") != before {
		t.Fatalf("the same lines were judged again (%d calls)", cs.count("tiny:4b")-before)
	}
	rows, _ := a.JudgementRecords(JudgementFilter{Kind: "handoff-summary"})
	if len(rows.Records) < 2 || rows.Records[len(rows.Records)-1].EffectKind != "chat" {
		t.Fatalf("records %+v", rows.Records)
	}
	one, _ := a.JudgementRecords(JudgementFilter{Kind: "handoff-summary", EffectID: "chat-1", Used: "judge"})
	if len(one.Records) != 1 {
		t.Fatalf("the judged call is recorded %d times", len(one.Records))
	}
	// More lines than the cap: the judge is shown as many as fit.
	many := handoffLines(300)
	_, shownMany, okMany := a.judgeHandoffDigest("chat-2", many, 5000)
	if !okMany || shownMany == 0 || shownMany >= 300 {
		t.Fatalf("shown %d of 300 (ok %v)", shownMany, okMany)
	}
	rec, _ := a.JudgementRecords(JudgementFilter{EffectID: "chat-2", Kind: "handoff-summary"})
	if len(rec.Records) != 1 || len(rec.Records[0].Input) > 12000 {
		t.Fatalf("records %+v", rec.Records)
	}
	// A summary longer than the room it may take is refused, and so is a limit of
	// nothing.
	if _, _, ok := a.judgeHandoffDigest("chat-3", handoffLines(2), 20); ok {
		t.Fatal("a summary longer than its room was used")
	}
	if _, _, ok := a.judgeHandoffDigest("chat-3", handoffLines(2), 0); ok {
		t.Fatal("a call was made with no room")
	}
	// An applet context in a line never reaches the judge or the record.
	secret := "dinah-applet-v1:5QAHWLZE3I6FEG3ZDG634FNDCD.53G4C65FBRVPZX77KYPVNNDMAH"
	a.judgeHandoffDigest("chat-4", []string{"[Person] use " + secret + " please", "[Agent] done"}, 5000)
	rec, _ = a.JudgementRecords(JudgementFilter{EffectID: "chat-4", Kind: "handoff-summary"})
	if len(rec.Records) != 1 || strings.Contains(rec.Records[0].Input, "5QAHWLZE") || strings.Contains(rec.Records[0].Out, "5QAHWLZE") {
		t.Fatalf("a context reached the record: %+v", rec.Records)
	}
}
