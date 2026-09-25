package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// declareHoldsOn writes a dinah.holds block, given whole, into the workbench
// anchor.
func declareHoldsOn(t *testing.T, root, block string) {
	t.Helper()
	path := filepath.Join(soleBenchDir(t, root), bench.WorkbenchAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.SetRaw(bench.HoldsKey, bench.SplitLines(strings.TrimSuffix(block, "\n")))
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		t.Fatalf("write the workbench anchor: %v", err)
	}
}

// needsBlock declares needs, holding the carrier until the named card
// reaches a done column.
const needsBlock = "dinah.holds:\n  kinds:\n    needs:\n      held: carrier\n"

// TestShowPrintsEachWaitLine is the show half of dinah-608/criteria/13: one
// line per ground, in the awaiting and lagging forms for a start, a finish and
// a named column, the not-before date after an awaiting line, and the one-day
// and many-day forms of a lag.
func TestShowPrintsEachWaitLine(t *testing.T) {
	s := &session{r: msg.For(msg.Base)}
	view := &verb.CardView{WaitsOn: []verb.WaitView{
		{Ref: "fx-1", WaitsFor: contract.HoldWaitsStart},
		{Ref: "fx-2", WaitsFor: contract.HoldWaitsFinish},
		{Ref: "fx-3", WaitsFor: contract.HoldWaitsFinish, FinishAt: "acceptance", FinishAtTitle: "Acceptance"},
		{Ref: "fx-4", WaitsFor: contract.HoldWaitsFinish, NotBefore: "2026-11-01"},
		{Ref: "fx-5", WaitsFor: contract.HoldWaitsStart, LagDays: 1, Reached: "2026-10-02", Until: "2026-10-03"},
		{Ref: "fx-6", WaitsFor: contract.HoldWaitsFinish, LagDays: 7, Reached: "2026-10-02", Until: "2026-10-09"},
		{Ref: "fx-7", WaitsFor: contract.HoldWaitsFinish, FinishAt: "acceptance", FinishAtTitle: "Acceptance", LagDays: 2, Reached: "2026-10-02", Until: "2026-10-04"},
	}}
	want := []string{
		"  waits on: fx-1 to start",
		"  waits on: fx-2 to finish",
		"  waits on: fx-3 to reach Acceptance",
		"  waits on: fx-4 to finish, not before 2026-11-01",
		"  waits until 2026-10-03: 1 day after fx-5 started",
		"  waits until 2026-10-09: 7 days after fx-6 finished",
		"  waits until 2026-10-04: 2 days after fx-7 reached Acceptance",
	}
	if got := s.waitLines(view); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("the wait lines read\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestTheScheduleCellNamesTheCardsWaitedOn is the listing half of
// dinah-608/criteria/13: the waiting phrase alone, after the not-yet phrase,
// and after a higher condition with the not-yet phrase between them.
func TestTheScheduleCellNamesTheCardsWaitedOn(t *testing.T) {
	s := &session{r: msg.For(msg.Base)}
	waits := []verb.WaitView{{Ref: "fx-1"}, {Ref: "fx-2"}, {Ref: "fx-1"}}
	for _, c := range []struct {
		name string
		view verb.CardView
		want string
	}{
		{name: "waiting alone", view: verb.CardView{Schedule: []string{contract.ScheduleWaiting}, WaitsOn: waits}, want: "waiting on fx-1, fx-2"},
		{name: "after not yet", view: verb.CardView{StartAfter: "2026-11-01", Schedule: []string{contract.ScheduleNotYet, contract.ScheduleWaiting}, WaitsOn: waits}, want: "not before 2026-11-01, waiting on fx-1, fx-2"},
		{name: "after a higher condition", view: verb.CardView{Due: "2026-10-01", StartAfter: "2026-11-01", Schedule: []string{contract.ScheduleOverdue, contract.ScheduleNotYet, contract.ScheduleWaiting}, WaitsOn: waits}, want: "overdue 2026-10-01, not before 2026-11-01, waiting on fx-1, fx-2"},
		{name: "not yet alone", view: verb.CardView{StartAfter: "2026-11-01", Schedule: []string{contract.ScheduleNotYet}}, want: "not before 2026-11-01"},
	} {
		if got := s.scheduleCell(&c.view); got != c.want {
			t.Errorf("%s: the cell reads %q, want %q", c.name, got, c.want)
		}
	}
}

// TestNextPrintsTheWaitingSentence is the text half of dinah-608/criteria/7:
// waiting alone, waiting beside a date, above the tier beside both, and the
// more form past three cards, on next and on prime.
func TestNextPrintsTheWaitingSentence(t *testing.T) {
	english := msg.For(msg.Base)
	for _, c := range []struct {
		name  string
		offer verb.Offer
		want  string
	}{
		{name: "waiting", offer: verb.Offer{Title: "Intake", Waiting: true, WaitingOn: []string{"fx-1"}}, want: english.T("next.waiting", "cards", "fx-1")},
		{name: "waiting beside a date", offer: verb.Offer{Title: "Intake", NotYet: true, StartableFrom: "2026-11-01", Waiting: true, WaitingOn: []string{"fx-1", "fx-2"}}, want: "ready, waiting on fx-1, fx-2"},
		{name: "above the tier", offer: verb.Offer{Title: "Intake", AboveTier: true, NotYet: true, StartableFrom: "2026-11-01", Waiting: true, WaitingOn: []string{"fx-1"}}, want: english.T("next.above-tier")},
		{name: "more than three", offer: verb.Offer{Title: "Intake", Waiting: true, WaitingOn: []string{"fx-1", "fx-2", "fx-3", "fx-4", "fx-5"}}, want: "ready, waiting on fx-1, fx-2, fx-3 and 2 more"},
	} {
		var out bytes.Buffer
		s := &session{r: english, out: &out, width: 200}
		s.renderOffers([]verb.Offer{c.offer})
		if !strings.Contains(out.String(), c.want) {
			t.Errorf("%s: next prints\n%s\nwithout %q", c.name, out.String(), c.want)
		}
		out.Reset()
		s.renderPrimeReady([]verb.Offer{c.offer})
		if !strings.Contains(out.String(), "Intake: "+c.want) {
			t.Errorf("%s: prime prints\n%s\nwithout %q", c.name, out.String(), c.want)
		}
	}
}

// TestAWaitingCardReadsTheSameAtEveryTerminalSurface drives the wedding
// planner's workbench through the binary: show, list, query, next in text and
// in both machine forms, pull, the claim warning, the cycle warning and the
// check finding.
func TestAWaitingCardReadsTheSameAtEveryTerminalSurface(t *testing.T) {
	root := newBench(t)
	t.Setenv("COLUMNS", "200")
	english := msg.For(msg.Base)
	declareHoldsOn(t, root, needsBlock)
	venue := fileCard(t, root, "Book the venue", "--start-after", dayFrom(37))
	caterer := fileCard(t, root, "Choose the caterer")
	invitations := fileCard(t, root, "Send the invitations", "--due", dayFrom(80))
	linked := mustRunHere(t, root, "link", invitations, "needs", venue)
	if strings.Contains(linked.out+linked.errw, "cycle") {
		t.Errorf("a link closing no cycle warns:\n%s", linked.out)
	}
	shown := mustRunHere(t, root, "show", invitations, "--fields", "card")
	if want := "  waits on: " + venue + " to finish, not before " + dayFrom(37); !strings.Contains(shown.out, "\n"+want+"\n") {
		t.Errorf("show does not print %q:\n%s", want, shown.out)
	}
	cell := english.T("schedule.cell.waiting", "cards", venue)
	for _, argv := range [][]string{{"list", "intake"}, {"query", "schedule:waiting"}} {
		_, rows := scheduleTable(t, mustRunHere(t, root, argv...).out)
		if rows[invitations]["Schedule"] != cell {
			t.Errorf("%v draws the cell %q, want %q", argv, rows[invitations]["Schedule"], cell)
		}
	}
	if first := mustRunHere(t, root, "pull", "doing"); !strings.HasPrefix(first.out, caterer+" ") {
		t.Errorf("the first pull did not take the caterer, the one card nothing holds:\n%s", first.out)
	}
	waiting := english.T("next.waiting", "cards", venue)
	if next := mustRunHere(t, root, "next", "intake"); !strings.Contains(next.out, waiting) {
		t.Errorf("next does not print %q:\n%s", waiting, next.out)
	}
	if next := mustRunHere(t, root, "next", "intake", "--json"); !strings.Contains(next.out, `"waiting": true`) || !strings.Contains(next.out, `"waiting_on": [`) || !strings.Contains(next.out, `"not_yet": true`) {
		t.Errorf("next --json reads:\n%s", next.out)
	}
	compact := mustRunHere(t, root, "--format", "compact", "next", "intake")
	if !strings.HasPrefix(compact.out, "fmt|compact|6\n") || !strings.Contains(compact.out, "|"+dayFrom(37)+"|1|"+venue+"\n") {
		t.Errorf("the compact off record reads:\n%s", compact.out)
	}
	pulled := runCLI(t, root, "pull", "doing")
	if want := english.T("answer.pull.waiting.named", "upstream", "Intake", "destination", "Doing", "cards", venue); pulled.code != 0 || !strings.Contains(pulled.out, want) {
		t.Errorf("pull exits %d and does not print %q:\n%s%s", pulled.code, want, pulled.out, pulled.errw)
	}
	// A claim at a station, after moving the card to one, warns naming the
	// venue; the intake column refuses the claim on its own account.
	cycle := mustRunHere(t, root, "link", venue, "needs", invitations)
	if want := english.T("warn.hold-cycle", "detail", venue+", "+invitations); !strings.Contains(cycle.errw, want) {
		t.Errorf("the closing link does not print %q:\n%s", want, cycle.out)
	}
	checked := runCLI(t, root, "check")
	if want := english.T("check.hold-cycle", "detail", venue+", "+invitations); checked.code != 5 || !strings.Contains(checked.out, want) {
		t.Errorf("check exits %d without %q:\n%s", checked.code, want, checked.out)
	}
	mustRunHere(t, root, "unlink", venue, "needs", invitations)
	mustRunHere(t, root, "move", invitations, "doing")
	mustRunHere(t, root, "move", invitations, "intake")
	declareHoldsOn(t, root, "dinah.holds:\n  start_at: done\n  kinds:\n    needs:\n      held: carrier\n")
	malformed := runCLI(t, root, "check")
	if !strings.Contains(malformed.out, english.T("check.holds-malformed", "detail", "dinah.holds start_at a done column")) {
		t.Errorf("check does not report the done commitment column:\n%s", malformed.out)
	}
}

// TestAClaimNamesTheCardsItWaitedOn is the terminal half of
// dinah-608/criteria/9: the warning a claim by name carries, in its text.
func TestAClaimNamesTheCardsItWaitedOn(t *testing.T) {
	root := newBench(t)
	english := msg.For(msg.Base)
	// A station before the commitment column is where a claim is admitted
	// on a card that has not started.
	mustRunHere(t, root, "column", "new", "Triage", "--before", "doing")
	declareHoldsOn(t, root, "dinah.holds:\n  start_at: doing\n  kinds:\n    needs:\n      held: carrier\n")
	holder := fileCard(t, root, "holder")
	held := fileCard(t, root, "held")
	mustRunHere(t, root, "link", held, "needs", holder)
	refused := runCLI(t, root, "claim", held)
	if refused.code == 0 || strings.Contains(refused.errw+refused.out, "waiting") {
		t.Errorf("a claim in the intake column answered %d:\n%s%s", refused.code, refused.out, refused.errw)
	}
	mustRunHere(t, root, "move", held, "triage")
	claimed := mustRunHere(t, root, "claim", held)
	if want := english.T("warn.waiting-on", "detail", holder); !strings.Contains(claimed.errw, want) {
		t.Errorf("the claim does not print %q:\n%s", want, claimed.out)
	}
	if shown := mustRunHere(t, root, "show", held, "--fields", "card"); strings.Contains(shown.out, "waits on") {
		t.Errorf("the claimed card still waits:\n%s", shown.out)
	}
	mustRunHere(t, root, "release", held)
	if shown := mustRunHere(t, root, "show", held, "--fields", "card"); !strings.Contains(shown.out, "  waits on: "+holder+" to finish") {
		t.Errorf("released in Triage the card does not wait again:\n%s", shown.out)
	}
}

// TestTheHoldsMigrationAdviceIsACommandThatWorks follows the sentence
// check.holds-below-format prints, on the pattern of the scheduling
// migration's own test: a format-10 workbench declaring a usable rule is
// reported, the preview writes nothing, and the command the sentence names
// stamps the format, after which the finding is gone. It is the terminal half
// of dinah-608/criteria/15.
func TestTheHoldsMigrationAdviceIsACommandThatWorks(t *testing.T) {
	english := msg.For(msg.Base)
	root := newBench(t)
	dir := soleBenchDir(t, root)
	stampFormat(t, dir, bench.HoldsFormat-1)
	if clean := runCLI(t, root, "check"); clean.code != 0 {
		t.Fatalf("check exits %d on a format-10 workbench declaring no hold:\n%s", clean.code, clean.out)
	}
	declareHoldsOn(t, root, needsBlock)
	advice := english.T("check.holds-below-format", "detail", "10")
	reported := runCLI(t, root, "check")
	if reported.code != 5 || !strings.Contains(reported.out, advice) {
		t.Fatalf("check exits %d and does not print %q:\n%s", reported.code, advice, reported.out)
	}
	before, _ := bench.ReadText(filepath.Join(dir, bench.WorkbenchAnchor))
	preview := runCLI(t, root, "check", "--migrate-holds")
	if want := english.T("check.format-would-stamp", "from", "10", "format", "11"); !strings.Contains(preview.out, want) {
		t.Errorf("the preview does not print %q:\n%s", want, preview.out)
	}
	if after, _ := bench.ReadText(filepath.Join(dir, bench.WorkbenchAnchor)); after != before {
		t.Error("the preview wrote the anchor")
	}
	opened := strings.Index(advice, "`")
	closed := strings.LastIndex(advice, "`")
	if opened < 0 || closed <= opened {
		t.Fatalf("the advice names no command in backticks: %q", advice)
	}
	command := strings.Fields(strings.TrimPrefix(advice[opened+1:closed], "dinah "))
	followed := runCLI(t, root, command...)
	if followed.code != 0 || !strings.Contains(followed.out, english.T("check.format-stamped", "format", "11")) {
		t.Fatalf("following the advice %v exits %d:\n%s%s", command, followed.code, followed.out, followed.errw)
	}
	after, _ := bench.ReadText(filepath.Join(dir, bench.WorkbenchAnchor))
	if after != strings.Replace(before, "\nformat: 10\n", "\nformat: 11\n", 1) {
		t.Errorf("the stamp wrote more than the format line:\n%s", after)
	}
	if again := runCLI(t, root, "check"); again.code != 0 || strings.Contains(again.out, advice) {
		t.Errorf("check exits %d after the stamp:\n%s", again.code, again.out)
	}
	current := runCLI(t, root, "check", "--migrate-holds", "--yes")
	if want := english.T("check.format-current", "format", "11"); !strings.Contains(current.out, want) || current.code != 0 {
		t.Errorf("the second run exits %d and does not print %q:\n%s", current.code, want, current.out)
	}
}

// TestTheExplanationCountsTheCardsWaiting is the terminal half of
// dinah-608/criteria/14: --explain names how many cards wait on a card, and
// says no link kind holds anything on a workbench declaring none.
func TestTheExplanationCountsTheCardsWaiting(t *testing.T) {
	root := newBench(t)
	t.Setenv("COLUMNS", "200")
	english := msg.For(msg.Base)
	holder := fileCard(t, root, "holder")
	carryToDoing(t, root, holder)
	held := fileCard(t, root, "held")
	mustRunHere(t, root, "link", held, "needs", holder)
	if drawn := mustRunHere(t, root, "view", "agenda", "--explain"); !strings.Contains(drawn.out, english.T("view.urgency.explain.blocks-others")) {
		t.Errorf("with no dinah.holds the explanation does not say so:\n%s", drawn.out)
	}
	declareHoldsOn(t, root, needsBlock)
	if drawn := mustRunHere(t, root, "view", "agenda", "--explain"); !strings.Contains(drawn.out, english.TN("view.urgency.explain.blocks-others.counted", 1)) {
		t.Errorf("the explanation does not count the waiting card:\n%s", drawn.out)
	}
}
