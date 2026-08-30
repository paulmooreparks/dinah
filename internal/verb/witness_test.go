package verb

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// handEdit rewrites a card's column in its anchor and nowhere else, which is
// what a person with an editor does and what no verb of this tool does. The
// journal is left saying what it already said, so the card is diverged the
// moment this returns.
func (h *harness) handEdit(ref, column string) {
	h.t.Helper()
	path := h.card(ref).AnchorPath()
	text, err := os.ReadFile(path)
	if err != nil {
		h.t.Fatalf("read %s: %v", path, err)
	}
	card := h.card(ref)
	was := "column: " + card.Column
	if !strings.Contains(string(text), was) {
		h.t.Fatalf("the anchor at %s carries no %q", path, was)
	}
	edited := strings.Replace(string(text), was, "column: "+column, 1)
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		h.t.Fatalf("write %s: %v", path, err)
	}
	h.reopen()
}

// witnesses returns the manual_correction lines of a card's journal, in file
// order, together with their positions in it, so a test can assert both what
// was written and where it stands relative to the verb's own line.
func (h *harness) witnesses(ref string) (lines []bench.Event, positions []int) {
	h.t.Helper()
	for i, ev := range h.events(ref) {
		if ev.Event == contract.EventManualCorrection {
			lines = append(lines, ev)
			positions = append(positions, i)
		}
	}
	return lines, positions
}

// TestAClaimWitnessesAHandEditedPositionBeforeItsOwnEffect is the verb path's
// own half of the format's promise. The card is edited with no verb involved,
// and the first verb that touches it records what it found before it records
// what it did.
func TestAClaimWitnessesAHandEditedPositionBeforeItsOwnEffect(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("A hand-edited card")
	h.handEdit(ref, doing)

	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})

	lines, positions := h.witnesses(ref)
	if len(lines) != 1 {
		t.Fatalf("the journal carries %d witnesses, wanted exactly one", len(lines))
	}
	if lines[0].To != doing {
		t.Errorf("the witness records to %q, wanted the column the anchor named at the moment of the touch", lines[0].To)
	}
	if lines[0].From != aftercare {
		t.Errorf("the witness records from %q, wanted the column the journal believed", lines[0].From)
	}
	if lines[0].Actor != "alka" {
		t.Errorf("the witness records actor %q, wanted whoever ran the verb", lines[0].Actor)
	}
	events := h.events(ref)
	claimed := -1
	for i, ev := range events {
		if ev.Event == contract.EventClaimed {
			claimed = i
		}
	}
	if claimed < 0 {
		t.Fatal("the claim wrote no claimed event, so this test measured nothing")
	}
	if positions[0] > claimed {
		t.Errorf("the witness stands at line %d and the claim's own event at line %d, so the record reads as though the edit happened after the claim", positions[0], claimed)
	}
}

// TestAPullWitnessesAHandEditedPositionBeforeItsOwnEffect is the same
// assertion on the other lock-holding entry point, which reaches its effect
// through a transaction of its own rather than through Do.
func TestAPullWitnessesAHandEditedPositionBeforeItsOwnEffect(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("A hand-edited card")
	h.handEdit(ref, intake)

	response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", Column: doing})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("pull: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()

	lines, positions := h.witnesses(ref)
	if len(lines) != 1 {
		t.Fatalf("the journal carries %d witnesses, wanted exactly one", len(lines))
	}
	if lines[0].To != intake {
		t.Errorf("the witness records to %q, wanted the column the anchor named at the moment of the touch", lines[0].To)
	}
	if lines[0].From != aftercare {
		t.Errorf("the witness records from %q, wanted the column the journal believed", lines[0].From)
	}
	moved := -1
	for i, ev := range h.events(ref) {
		if ev.Event == contract.EventMoved {
			moved = i
		}
	}
	if moved < 0 {
		t.Fatal("the pull wrote no moved event, so this test measured nothing")
	}
	if positions[0] > moved {
		t.Errorf("the witness stands at line %d and the pull's own moved event at line %d, so the record reads as though the edit happened after the pull", positions[0], moved)
	}
}

// TestAWitnessedCardIsNoLongerReportedAsDivergedByCheck closes the loop the
// verb path opens: the check that reported the divergence stops reporting it
// once a touch has witnessed it, with no repair run of its own.
func TestAWitnessedCardIsNoLongerReportedAsDivergedByCheck(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("A hand-edited card")
	h.handEdit(ref, doing)
	if _, found := finding(h.check(), bench.FindingPositionDiverges); !found {
		t.Fatal("the hand edit did not diverge the card, so the rest of this test proves nothing")
	}
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	if detail, found := finding(h.check(), bench.FindingPositionDiverges); found {
		t.Errorf("the witnessed card is still reported as diverged, believing %s", detail)
	}
}

// TestAWitnessThatCannotBeWrittenIsUnreachableAndStopsTheVerb pins the two
// halves of D-2 at once. A witness the disk refuses is reported as
// unreachable and never as refused, because divergence is not grounds to
// reject anybody's claim; and the verb's own effect never runs, because the
// witness stands ahead of it. Moving the call after the effect reddens the
// second half.
//
// The permission goes on the journal file rather than on the card's
// directory. The append opens that file for writing, so its own mode is what
// governs the write on every platform this tool ships to, where a directory's
// mode governs only on POSIX. A user who ignores permissions is skipped
// rather than asserted at.
func TestAWitnessThatCannotBeWrittenIsUnreachableAndStopsTheVerb(t *testing.T) {
	if runtime.GOOS != "windows" && os.Geteuid() == 0 {
		t.Skip("root is not stopped by a file permission")
	}
	h := newHarness(t)
	ref := h.ready("A hand-edited card")
	h.handEdit(ref, doing)
	journal := h.card(ref).JournalPath()
	if err := os.Chmod(journal, 0o444); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(journal, 0o644) })

	response := h.do(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	if response.Outcome != contract.OutcomeUnreachable {
		t.Errorf("the claim answered %s, wanted %s: a disk that refuses the witness is unreachable, not a refusal", response.Outcome, contract.OutcomeUnreachable)
	}
	if card := h.card(ref); card.State != contract.StateReady || card.Holder != "" {
		t.Errorf("the card is %s held by %q, so the verb's own effect ran after the witness had already failed", card.State, card.Holder)
	}
}

// TestAReadThatLapsesAClaimWitnessesTheHandEditItFinds is D-6 in a test. A
// card whose only visitors are reads still reaches the lock-and-append path,
// through the lapse a read performs, and that is where the format's "next
// touch" has to hold or a diverged card nobody writes to is never witnessed
// at all.
func TestAReadThatLapsesAClaimWitnessesTheHandEditItFinds(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("A hand-edited card")
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka", Expires: time.Hour})
	h.handEdit(ref, doing)
	h.advance(2 * time.Hour)

	if _, err := h.library.List(&Request{Verb: "ls", Actor: "bela"}); err != nil {
		t.Fatalf("ls: %v", err)
	}
	h.reopen()

	lines, positions := h.witnesses(ref)
	if len(lines) != 1 {
		t.Fatalf("the journal carries %d witnesses, wanted exactly one", len(lines))
	}
	if lines[0].To != doing {
		t.Errorf("the witness records to %q, wanted the column the anchor named", lines[0].To)
	}
	if lines[0].Actor != "bela" {
		t.Errorf("the witness records actor %q, wanted whoever ran the read", lines[0].Actor)
	}
	expired := -1
	for i, ev := range h.events(ref) {
		if ev.Event == contract.EventExpired {
			expired = i
		}
	}
	if expired < 0 {
		t.Fatal("the read lapsed no claim, so this test measured nothing")
	}
	if positions[0] > expired {
		t.Errorf("the witness stands at line %d and the lapse's own line at %d, so the witness did not run before the write it precedes", positions[0], expired)
	}
}

// TestAReadThatLapsesNothingWitnessesNothing is the other side of the same
// placement, and it is what keeps the reach of D-6 honest. lapseRead returns
// before it takes the lock for a card whose claim has not expired, so a
// diverged card no claim ever lapsed on is left for check --witness or for a
// touch that writes.
func TestAReadThatLapsesNothingWitnessesNothing(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("A hand-edited card")
	h.handEdit(ref, doing)

	if _, err := h.library.List(&Request{Verb: "ls", Actor: "bela"}); err != nil {
		t.Fatalf("ls: %v", err)
	}
	h.reopen()

	if lines, _ := h.witnesses(ref); len(lines) != 0 {
		t.Errorf("a read that lapsed nothing wrote %d witnesses", len(lines))
	}
}
