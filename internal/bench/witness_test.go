package bench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// witnessDefinition is a bench of two columns, which is the smallest one a
// divergence can be stated on: a card has to be able to stand in a column its
// journal does not name.
const witnessDefinition = `---
format: 1
profile: dinah-core/0.7
title: Fixture
slug: fx
operator: alka
columns:
  - b00000000001
  - b00000000002
---
Standing text.
`

const doingColumn = `---
title: Doing
slug: doing
kind: work
---
Column text.
`

const doneColumn = `---
title: Done
slug: done
kind: done
---
Column text.
`

// divergedCard stands in Done and carries a journal that says Doing, which is
// what an editor leaves behind.
const divergedCard = `---
title: A card
number: 1
column: b00000000002
state: ready
---
Framing.
`

const divergedJournal = `{"ts":"2026-08-17T09:00:00Z","event":"created","actor":"alka","title":"A card","to":"b00000000001","to_title":"Doing"}
`

// newWitnessFixture writes the two-column bench and its one diverged card.
func newWitnessFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, WorkbenchAnchor), witnessDefinition)
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), doingColumn)
	write(t, filepath.Join(root, ColumnsDir, "b00000000002", ColumnAnchor), doneColumn)
	write(t, filepath.Join(root, CardsDir, "c00000000001", CardAnchor), divergedCard)
	write(t, filepath.Join(root, CardsDir, "c00000000001", JournalName), divergedJournal)
	return root
}

// journalEvents reads one card's journal back, which is how every assertion
// below reads what was written rather than what the writer said it wrote.
func journalEvents(t *testing.T, root, id string) []Event {
	t.Helper()
	events, torn, err := ReadJournal(filepath.Join(root, CardsDir, id, JournalName))
	if err != nil {
		t.Fatalf("read the journal of %s: %v", id, err)
	}
	if torn {
		t.Fatalf("the journal of %s ends in a partial line", id)
	}
	return events
}

// TestWitnessDivergenceRecordsTheEditItFound asserts the four fields of the
// line the witness writes, against a fixture whose believed and current
// columns differ in both identifier and title. A writer that swapped from and
// to, dropped a title, or put a value under the wrong member fails here,
// which presence-checking the four keys would not.
func TestWitnessDivergenceRecordsTheEditItFound(t *testing.T) {
	root := newWitnessFixture(t)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	card, err := LoadCard(opened.CardsRoot(), "c00000000001")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	wrote, err := opened.WitnessDivergence("alka", "2026-08-18T09:00:00Z", card)
	if err != nil {
		t.Fatalf("witness: %v", err)
	}
	if !wrote {
		t.Fatal("the witness reported writing nothing for a card whose anchor and journal disagree")
	}
	events := journalEvents(t, root, "c00000000001")
	if len(events) != 2 {
		t.Fatalf("wanted the created line and one witness, got %d lines", len(events))
	}
	got := events[1]
	if got.Event != contract.EventManualCorrection {
		t.Errorf("the appended line is a %s event, wanted %s", got.Event, contract.EventManualCorrection)
	}
	if got.From != "b00000000001" {
		t.Errorf("from is %q, wanted the column the journal believed", got.From)
	}
	if got.FromTitle != "Doing" {
		t.Errorf("from_title is %q, wanted Doing", got.FromTitle)
	}
	if got.To != "b00000000002" {
		t.Errorf("to is %q, wanted the column the anchor records", got.To)
	}
	if got.ToTitle != "Done" {
		t.Errorf("to_title is %q, wanted Done", got.ToTitle)
	}
	if got.Actor != "alka" {
		t.Errorf("actor is %q, wanted the actor the caller passed", got.Actor)
	}
	if got.TS != "2026-08-18T09:00:00Z" {
		t.Errorf("ts is %q, wanted the stamp the caller passed", got.TS)
	}
}

// TestWitnessDivergenceWritesNothingWhenTheTwoAgree is the other half: the
// witness is a repair and not a heartbeat, so a card whose journal replays to
// the column its anchor names gains no line at all.
func TestWitnessDivergenceWritesNothingWhenTheTwoAgree(t *testing.T) {
	root := newWitnessFixture(t)
	write(t, filepath.Join(root, CardsDir, "c00000000001", CardAnchor),
		strings.Replace(divergedCard, "column: b00000000002", "column: b00000000001", 1))
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	card, err := LoadCard(opened.CardsRoot(), "c00000000001")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	wrote, err := opened.WitnessDivergence("alka", "2026-08-18T09:00:00Z", card)
	if err != nil {
		t.Fatalf("witness: %v", err)
	}
	if wrote {
		t.Error("the witness wrote a line for a card whose anchor and journal already agree")
	}
	if events := journalEvents(t, root, "c00000000001"); len(events) != 1 {
		t.Errorf("the journal carries %d lines, wanted the created line alone", len(events))
	}
}

// TestAWitnessedCardIsNoLongerReportedAsDiverged pairs the writer with the
// checker: the position the witness records is the anchor's own, so replay
// catches up and the finding that provoked the repair goes away.
func TestAWitnessedCardIsNoLongerReportedAsDiverged(t *testing.T) {
	root := newWitnessFixture(t)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	findings, err := opened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !namesFinding(findings, FindingPositionDiverges) {
		t.Fatalf("the fixture is not diverged before the witness, so the rest of this test proves nothing: %v", findings)
	}
	card, err := LoadCard(opened.CardsRoot(), "c00000000001")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, err := opened.WitnessDivergence("alka", "2026-08-18T09:00:00Z", card); err != nil {
		t.Fatalf("witness: %v", err)
	}
	after, err := opened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if namesFinding(after, FindingPositionDiverges) {
		t.Errorf("the card is still reported as diverged after being witnessed: %v", after)
	}
}

// namesFinding reports whether a walk raised a finding of one key.
func namesFinding(findings []Finding, key string) bool {
	for _, finding := range findings {
		if finding.Key == key {
			return true
		}
	}
	return false
}

// TestWriteWitnessesTouchesOnlyTheCardsThatDiverge asserts the batch repair's
// whole answer: the identifiers it returns, the line it wrote, and the
// journal it left alone.
func TestWriteWitnessesTouchesOnlyTheCardsThatDiverge(t *testing.T) {
	root := newWitnessFixture(t)
	write(t, filepath.Join(root, CardsDir, "c00000000002", CardAnchor),
		strings.Replace(strings.Replace(divergedCard, "column: b00000000002", "column: b00000000001", 1), "number: 1", "number: 2", 1))
	write(t, filepath.Join(root, CardsDir, "c00000000002", JournalName), divergedJournal)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	witnessed, findings, err := opened.WriteWitnesses("alka", "2026-08-18T09:00:00Z")
	if err != nil {
		t.Fatalf("write witnesses: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("the repair raised %v on a workbench holding nothing in its way", findings)
	}
	if len(witnessed) != 1 || witnessed[0] != "c00000000001" {
		t.Fatalf("witnessed %v, wanted the diverging card alone", witnessed)
	}
	if events := journalEvents(t, root, "c00000000001"); len(events) != 2 || events[1].Event != contract.EventManualCorrection {
		t.Errorf("the diverging card's journal carries %d lines and no witness", len(events))
	}
	if events := journalEvents(t, root, "c00000000002"); len(events) != 1 {
		t.Errorf("the agreeing card's journal was written to: %d lines", len(events))
	}
}

// TestTheWitnessWalkStepsOverALockedCardAndFinishesTheWalk asserts the
// obstruction is reported rather than fatal. A lock another actor holds is
// ordinary, and a repair that abandoned the bench on the first one would
// leave every card after it unwitnessed with no account of why.
func TestTheWitnessWalkStepsOverALockedCardAndFinishesTheWalk(t *testing.T) {
	root := newWitnessFixture(t)
	second := filepath.Join(root, CardsDir, "c00000000002")
	write(t, filepath.Join(second, CardAnchor), strings.Replace(divergedCard, "number: 1", "number: 2", 1))
	write(t, filepath.Join(second, JournalName), divergedJournal)
	held, err := json.Marshal(LockRecord{Actor: "someone-else", PID: os.Getpid(), TS: "2026-08-18T08:00:00Z"})
	if err != nil {
		t.Fatalf("marshal the lock record: %v", err)
	}
	write(t, filepath.Join(root, CardsDir, "c00000000001", LockName), string(held)+"\n")

	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	witnessed, findings, err := opened.WriteWitnesses("alka", "2026-08-18T09:00:00Z")
	if err != nil {
		t.Fatalf("a locked card should not end the walk: %v", err)
	}
	named := false
	for _, finding := range findings {
		if finding.Key == FindingWitnessLocked && finding.Detail == "c00000000001" {
			named = true
		}
	}
	if !named {
		t.Errorf("the locked card is not reported under %s: %v", FindingWitnessLocked, findings)
	}
	if len(witnessed) != 1 || witnessed[0] != "c00000000002" {
		t.Errorf("witnessed %v, wanted the second card, which the walk only reaches by carrying on past the lock", witnessed)
	}
	if events := journalEvents(t, root, "c00000000001"); len(events) != 1 {
		t.Errorf("the locked card was written to anyway: %d lines", len(events))
	}
}

// TestArrivalReadsAWitnessedCorrectionAsAnArrival pins the queue order a
// witness would otherwise wreck. A card whose position was reconciled rather
// than moved carries no move naming the column it stands in, so a reader that
// counted moves alone would report the zero time for it and sort it ahead of
// every card that arrived by one.
func TestArrivalReadsAWitnessedCorrectionAsAnArrival(t *testing.T) {
	root := newWitnessFixture(t)
	moved := filepath.Join(root, CardsDir, "c00000000002")
	write(t, filepath.Join(moved, CardAnchor), strings.Replace(divergedCard, "number: 1", "number: 2", 1))
	write(t, filepath.Join(moved, JournalName), divergedJournal+
		`{"ts":"2026-08-18T09:00:00Z","event":"moved","actor":"alka","from":"b00000000001","from_title":"Doing","to":"b00000000002","to_title":"Done"}`+"\n")
	witnessed := filepath.Join(root, CardsDir, "c00000000001")
	write(t, filepath.Join(witnessed, JournalName), divergedJournal+
		`{"ts":"2026-08-18T10:00:00Z","event":"manual_correction","actor":"alka","from":"b00000000001","from_title":"Doing","to":"b00000000002","to_title":"Done"}`+"\n")

	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	cards, err := opened.Cards()
	if err != nil {
		t.Fatalf("cards: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("wanted two cards, got %d", len(cards))
	}
	sort.SliceStable(cards, func(i, j int) bool { return ByArrival(cards[i], cards[j]) })
	if cards[0].ID != "c00000000002" {
		t.Errorf("the queue leads with %s, wanted the card that arrived by a move at the earlier stamp", cards[0].ID)
	}
	if cards[1].Arrival().IsZero() {
		t.Error("the witnessed card reports the zero time, so it would jump the queue whatever its stamp said")
	}
}
