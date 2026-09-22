package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// exitHoldDefinition is a four-column flow whose working station holds a card
// on the way out. Intake stands before it and Review after it, so one fixture
// reaches a forward departure and a regressive one without being rebuilt.
const exitHoldDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Exit hold",
  "columns": [
    { "id": "f00000000001", "title": "Intake", "kind": "intake",
      "instructions": "Intake instructions.\n" },
    { "id": "f00000000002", "title": "Doing", "kind": "work",
      "gate_items": "out", "instructions": "Doing instructions.\n" },
    { "id": "f00000000003", "title": "Review", "kind": "work",
      "instructions": "Review instructions.\n" },
    { "id": "f00000000004", "title": "Done", "kind": "done",
      "instructions": "Done instructions.\n" }
  ]
}`

// heldOnTheWayOut builds the flow above, files one item of the given kind
// against the holding station by that station's slug, and leaves the card
// standing there. The slug rather than the identifier is deliberate: it
// exercises dinah-473's own resolution on the way in, which is the spelling a
// person actually types.
func heldOnTheWayOut(t *testing.T, kind string) (string, string) {
	t.Helper()
	root := newBenchFromDefinition(t, exitHoldDefinition)
	if got := runCLI(t, root, "add", "Write the release notes"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "doing"); got.code != 0 {
		t.Fatalf("move to doing: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "file", "--column", "doing", "fx-1", kind, "Somebody has to settle this."); got.code != 0 {
		t.Fatalf("file a %s: %d %s", kind, got.code, got.errw)
	}
	return root, soleItemID(t, root, "fx-1")
}

// TestAColumnHoldingOnTheWayOutRefusesOnlyTheForwardDeparture is dinah-571's
// own case, replacing dinah-484's AC-4 and AC-5 now that the direction is
// read. The forward departure is refused exactly as it always was, and the
// regressive one now passes with the item riding along unresolved: sending
// the card back upstream escapes nothing, because the item still names the
// station and the station holds it again the next time the card tries to
// leave forward.
//
// The item is then settled and the same forward move goes through, which is
// what stops this passing on a build that refuses every move out of the
// station whatever the card carries.
func TestAColumnHoldingOnTheWayOutRefusesOnlyTheForwardDeparture(t *testing.T) {
	root, item := heldOnTheWayOut(t, "acceptance_criterion")

	forward := runCLI(t, root, "move", "fx-1", "review")
	if forward.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("the forward departure exited %d, wanted %d", forward.code, contract.ExitCode(contract.OutcomeRefused))
	}
	if name := refusalNameOf(forward.errw); name != contract.UnresolvedItemExit {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.UnresolvedItemExit)
	}
	if !strings.Contains(forward.errw, item) {
		t.Errorf("the refusal names no item; wanted %s in:\n%s", item, forward.errw)
	}

	back := runCLI(t, root, "move", "fx-1", "intake")
	if back.code != 0 {
		t.Fatalf("the regressive departure was held: %d %s, so the exit hold still reads no direction", back.code, back.errw)
	}
	if state := soleItemState(t, root, "fx-1"); state != bench.ItemPending {
		t.Errorf("the item rode the regressive move as %q, wanted it to stand pending, unchanged", state)
	}

	// The card is carried back to the station and the same forward move is
	// tried again: the item still names it, so the station holds it again,
	// which is what proves the regressive move settled nothing.
	if got := runCLI(t, root, "move", "fx-1", "doing"); got.code != 0 {
		t.Fatalf("move back to doing: %d %s", got.code, got.errw)
	}
	stillForward := runCLI(t, root, "move", "fx-1", "review")
	if stillForward.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("the forward departure after the round trip exited %d, wanted %d", stillForward.code, contract.ExitCode(contract.OutcomeRefused))
	}
	if name := refusalNameOf(stillForward.errw); name != contract.UnresolvedItemExit {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.UnresolvedItemExit)
	}

	if got := runCLI(t, root, "verify", "fx-1/criteria/1", "--text", "the release notes are written, read against the fixture"); got.code != 0 {
		t.Fatalf("verify: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "review"); got.code != 0 {
		t.Fatalf("the departure was refused with the item settled: %d %s", got.code, got.errw)
	}
}

// TestAColumnHoldingOnTheWayOutReleasesEveryOtherCard is dinah-484 AC-6, and
// it is the negative control for the whole direction. Without it, a build
// refusing every departure from a column declaring the hold would pass every
// assertion above.
//
// Two shapes go through: a card carrying no item at all, and a card carrying
// one that names a different column, which is the shape that catches a build
// reading the hold and then ignoring which column the item names.
func TestAColumnHoldingOnTheWayOutReleasesEveryOtherCard(t *testing.T) {
	root := newBenchFromDefinition(t, exitHoldDefinition)
	for _, card := range []string{"Carries nothing", "Names another column"} {
		if got := runCLI(t, root, "add", card); got.code != 0 {
			t.Fatalf("add %s: %d %s", card, got.code, got.errw)
		}
	}
	if got := runCLI(t, root, "file", "--column", "review", "fx-2", "open_question", "Settled somewhere else."); got.code != 0 {
		t.Fatalf("file: %d %s", got.code, got.errw)
	}
	for _, card := range []string{"fx-1", "fx-2"} {
		if got := runCLI(t, root, "move", card, "doing"); got.code != 0 {
			t.Fatalf("move %s to doing: %d %s", card, got.code, got.errw)
		}
		if got := runCLI(t, root, "move", card, "intake"); got.code != 0 {
			t.Fatalf("%s was held on a regressive departure: %d %s", card, got.code, got.errw)
		}
		if got := runCLI(t, root, "move", card, "doing"); got.code != 0 {
			t.Fatalf("move %s back to doing: %d %s", card, got.code, got.errw)
		}
		if got := runCLI(t, root, "move", card, "review"); got.code != 0 {
			t.Fatalf("%s was held on a forward departure: %d %s", card, got.code, got.errw)
		}
	}
}

// TestTheOperatorCarriesACardPastTheExitHold is dinah-484 AC-15. No new
// override path is opened by this card: the marker is the one that already
// carries a card into a full column, it is refused to anybody but the operator
// by a row that runs ahead of every hold, and the move it carries is witnessed
// as an override on the card's own journal.
func TestTheOperatorCarriesACardPastTheExitHold(t *testing.T) {
	root, _ := heldOnTheWayOut(t, "open_question")

	if got := runCLI(t, root, "move", "fx-1", "review", "--override"); got.code != 0 {
		t.Fatalf("the operator's override was refused: %d %s", got.code, got.errw)
	}
	events := cardJournal(t, root, cardID(t, root, "fx-1"))
	last := bench.Event{}
	moves := 0
	for _, ev := range events {
		if ev.Event == contract.EventMoved {
			moves++
			last = ev
		}
	}
	if moves != 2 {
		t.Fatalf("wanted two moved events, the fixture's own and the override, got %d", moves)
	}
	if !last.Override {
		t.Errorf("the moved event carries no override flag: %+v", last)
	}
}

// TestANonOperatorIsRefusedTheOverrideAtTheExitHold is AC-15's second half.
// The marker is refused by the row canRoute already carries, ahead of the
// hold, so the answer is not-operator rather than the exit hold's own name,
// which is what says the protection is the existing gate on the marker and not
// something this card had to build.
func TestANonOperatorIsRefusedTheOverrideAtTheExitHold(t *testing.T) {
	root, _ := heldOnTheWayOut(t, "decision")
	t.Setenv("DINAH_ACTOR", "sam")

	got := runCLI(t, root, "move", "fx-1", "review", "--override")
	if got.code == 0 {
		t.Fatal("an owner who is not the operator carried a card past the exit hold")
	}
	if name := refusalNameOf(got.errw); name != contract.NotOperator {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.NotOperator)
	}
}

// TestTheOperatorRefusalAtTheExitHoldNamesOverride is dinah-571's own case.
// The operator is refused by the exit hold exactly as anybody else is, and
// the refusal he reads names --override as the way past his own station's
// item, because he is the one caller who may actually pass it. A non-operator
// reading the same refusal is told to settle the item and reads no mention of
// a flag refused to him.
func TestTheOperatorRefusalAtTheExitHoldNamesOverride(t *testing.T) {
	root, item := heldOnTheWayOut(t, "acceptance_criterion")

	operator := runCLI(t, root, "move", "fx-1", "review")
	if operator.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("the operator's forward departure exited %d, wanted %d", operator.code, contract.ExitCode(contract.OutcomeRefused))
	}
	if name := refusalNameOf(operator.errw); name != contract.UnresolvedItemExit {
		t.Fatalf("the refusal name is %s, wanted %s", name, contract.UnresolvedItemExit)
	}
	if !strings.Contains(operator.errw, "--override") {
		t.Errorf("the operator's own refusal does not name --override:\n%s", operator.errw)
	}

	root2, item2 := heldOnTheWayOut(t, "acceptance_criterion")
	t.Setenv("DINAH_ACTOR", "sam")
	other := runCLI(t, root2, "move", "fx-1", "review")
	if other.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("the non-operator's forward departure exited %d, wanted %d", other.code, contract.ExitCode(contract.OutcomeRefused))
	}
	if name := refusalNameOf(other.errw); name != contract.UnresolvedItemExit {
		t.Fatalf("the refusal name is %s, wanted %s", name, contract.UnresolvedItemExit)
	}
	if strings.Contains(other.errw, "--override") {
		t.Errorf("a non-operator's refusal names --override, which he cannot pass:\n%s", other.errw)
	}
	if !strings.Contains(operator.errw, item) || !strings.Contains(other.errw, item2) {
		t.Fatalf("one of the two refusals does not name its own item")
	}
}

// exitHoldWithEarlyDoneDefinition puts a done-kind column earlier in the
// flow's own order than the station that holds on the way out, so a move
// into it is regressive by position and still has to be held: it is a move
// into a done column, which the exit hold binds on the same terms as a
// forward departure.
const exitHoldWithEarlyDoneDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Early done",
  "columns": [
    { "id": "f10000000001", "title": "Intake", "kind": "intake" },
    { "id": "f10000000002", "title": "Archived", "kind": "done" },
    { "id": "f10000000003", "title": "Doing", "kind": "work", "gate_items": "out" },
    { "id": "f10000000004", "title": "Review", "kind": "work" }
  ]
}`

// TestAColumnHoldingOnTheWayOutHoldsADepartureIntoADoneColumn is dinah-571's
// own case: Archived stands earlier than Doing in the flow's own order, so a
// move from Doing to Archived is regressive by position alone, and the exit
// hold still binds it, because RegressiveDepartures's own reading (and this
// card's regressive) excludes a column of kind done regardless of where it
// stands.
func TestAColumnHoldingOnTheWayOutHoldsADepartureIntoADoneColumn(t *testing.T) {
	root := newBenchFromDefinition(t, exitHoldWithEarlyDoneDefinition)
	if got := runCLI(t, root, "add", "Write the release notes"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "doing"); got.code != 0 {
		t.Fatalf("move to doing: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "file", "--column", "doing", "fx-1", "acceptance_criterion", "Somebody has to settle this."); got.code != 0 {
		t.Fatalf("file: %d %s", got.code, got.errw)
	}
	item := soleItemID(t, root, "fx-1")

	refused := runCLI(t, root, "move", "fx-1", "archived")
	if refused.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("the move into the earlier done column exited %d, wanted %d", refused.code, contract.ExitCode(contract.OutcomeRefused))
	}
	if name := refusalNameOf(refused.errw); name != contract.UnresolvedItemExit {
		t.Errorf("the refusal name is %s, wanted %s, so a done column standing earlier was read as an ordinary regressive move", name, contract.UnresolvedItemExit)
	}
	if !strings.Contains(refused.errw, item) {
		t.Errorf("the refusal names no item; wanted %s in:\n%s", item, refused.errw)
	}
}

// entryOnlyDefinition declares the legacy true spelling, which is entry-only,
// on the working station, so a card meets the hold entering it from either
// side.
const entryOnlyDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Entry only",
  "columns": [
    { "id": "f20000000001", "title": "Intake", "kind": "intake" },
    { "id": "f20000000002", "title": "Doing", "kind": "work", "gate_items": true },
    { "id": "f20000000003", "title": "Review", "kind": "work" },
    { "id": "f20000000004", "title": "Done", "kind": "done" }
  ]
}`

// TestAnEntryHoldStillRefusesEntryFromEitherDirection is dinah-571's own
// case: the entry hold is untouched by this card, so it refuses a forward
// entry and, once the card is past it, a regressive entry back into the same
// station exactly as it always did.
func TestAnEntryHoldStillRefusesEntryFromEitherDirection(t *testing.T) {
	root := newBenchFromDefinition(t, entryOnlyDefinition)
	if got := runCLI(t, root, "add", "Write the release notes"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "file", "--column", "doing", "fx-1", "acceptance_criterion", "Somebody has to settle this."); got.code != 0 {
		t.Fatalf("file: %d %s", got.code, got.errw)
	}
	item := soleItemID(t, root, "fx-1")

	forwardEntry := runCLI(t, root, "move", "fx-1", "doing")
	if forwardEntry.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("the forward entry exited %d, wanted %d", forwardEntry.code, contract.ExitCode(contract.OutcomeRefused))
	}
	if name := refusalNameOf(forwardEntry.errw); name != contract.UnresolvedItem {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.UnresolvedItem)
	}
	if !strings.Contains(forwardEntry.errw, item) {
		t.Errorf("the refusal names no item; wanted %s in:\n%s", item, forwardEntry.errw)
	}

	if got := runCLI(t, root, "move", "fx-1", "doing", "--override"); got.code != 0 {
		t.Fatalf("the operator's override at entry: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "review"); got.code != 0 {
		t.Fatalf("the forward departure out of doing, which declares no exit hold: %d %s", got.code, got.errw)
	}

	backEntry := runCLI(t, root, "move", "fx-1", "doing")
	if backEntry.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("the regressive entry exited %d, wanted %d", backEntry.code, contract.ExitCode(contract.OutcomeRefused))
	}
	if name := refusalNameOf(backEntry.errw); name != contract.UnresolvedItem {
		t.Errorf("the refusal name is %s, wanted %s, so the entry hold stopped reading both directions", name, contract.UnresolvedItem)
	}
}

// routedExitHoldDefinition declares one route naming every column of the flow
// above, so a card filed on it still walks the same order, and the fixture
// isolates one question: does a route on the card change whether a
// regressive departure from an out-holding column passes.
const routedExitHoldDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Routed exit hold",
  "routes": {
    "main": ["f30000000001", "f30000000002", "f30000000003", "f30000000004"]
  },
  "columns": [
    { "id": "f30000000001", "title": "Intake", "kind": "intake" },
    { "id": "f30000000002", "title": "Doing", "kind": "work", "gate_items": "out" },
    { "id": "f30000000003", "title": "Review", "kind": "work" },
    { "id": "f30000000004", "title": "Done", "kind": "done" }
  ]
}`

// TestARoutedCardsRegressiveDepartureAlsoPasses is dinah-571's own case,
// answering the specification's own question about a routed card: regressive
// is read off the flow's own column order, the way dinah-542's loop-limit
// reading already is, and a route carries no order of its own for this
// question, so a card walking one is held and released on exactly the same
// terms as a card walking none.
func TestARoutedCardsRegressiveDepartureAlsoPasses(t *testing.T) {
	root := newBenchFromDefinition(t, routedExitHoldDefinition)
	if got := runCLI(t, root, "add", "Write the release notes"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "set", "fx-1", "route", "main"); got.code != 0 {
		t.Fatalf("set the route: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "doing"); got.code != 0 {
		t.Fatalf("move to doing: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "file", "--column", "doing", "fx-1", "acceptance_criterion", "Somebody has to settle this."); got.code != 0 {
		t.Fatalf("file: %d %s", got.code, got.errw)
	}

	back := runCLI(t, root, "move", "fx-1", "intake")
	if back.code != 0 {
		t.Fatalf("the routed card's regressive departure was held: %d %s", back.code, back.errw)
	}
	if state := soleItemState(t, root, "fx-1"); state != bench.ItemPending {
		t.Errorf("the item rode the regressive move as %q, wanted it to stand pending, unchanged", state)
	}

	forward := runCLI(t, root, "move", "fx-1", "doing")
	if forward.code != 0 {
		t.Fatalf("move back to doing: %d %s", forward.code, forward.errw)
	}
	stillForward := runCLI(t, root, "move", "fx-1", "review")
	if stillForward.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("the routed card's forward departure exited %d, wanted %d", stillForward.code, contract.ExitCode(contract.OutcomeRefused))
	}
	if name := refusalNameOf(stillForward.errw); name != contract.UnresolvedItemExit {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.UnresolvedItemExit)
	}
}

// orderingDefinition puts a buffer directly after the station that holds on
// the way out. A buffer takes no work up, so a move into it carrying a held
// card is refused by a row that runs after the exit hold inside canLand, which
// makes this the fixture that can tell the two orders apart.
const orderingDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Ordering",
  "columns": [
    { "id": "f00000000001", "title": "Intake", "kind": "intake",
      "instructions": "Intake instructions.\n" },
    { "id": "f00000000002", "title": "Doing", "kind": "work",
      "gate_items": "out", "instructions": "Doing instructions.\n" },
    { "id": "f00000000003", "title": "Waiting", "kind": "dinah.buffer",
      "instructions": "Waiting instructions.\n" },
    { "id": "f00000000004", "title": "Done", "kind": "done",
      "instructions": "Done instructions.\n" }
  ]
}`

// TestTheExitHoldAnswersAheadOfTheDestinationRows is dinah-484 AC-14, and it
// pins the check's position by construction rather than by reading the source.
// The card fails two rows at once: the departure holds it, and the destination
// takes no work up while the card is held. The exit hold runs first, so its
// name is the one that comes back, and a build running the same check after
// the three destination rows would answer the other name here while passing
// every behavioural case above.
func TestTheExitHoldAnswersAheadOfTheDestinationRows(t *testing.T) {
	root := newBenchFromDefinition(t, orderingDefinition)
	if got := runCLI(t, root, "add", "Write the release notes"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "doing"); got.code != 0 {
		t.Fatalf("move to doing: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "claim", "fx-1"); got.code != 0 {
		t.Fatalf("claim: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "file", "--column", "doing", "fx-1", "acceptance_criterion", "Somebody has to settle this."); got.code != 0 {
		t.Fatalf("file: %d %s", got.code, got.errw)
	}
	item := soleItemID(t, root, "fx-1")

	refused := runCLI(t, root, "move", "fx-1", "waiting")
	if refused.code == 0 {
		t.Fatal("a move failing both the exit hold and the destination's own row succeeded")
	}
	if name := refusalNameOf(refused.errw); name != contract.UnresolvedItemExit {
		t.Errorf("the refusal name is %s, wanted %s, so a destination row answered ahead of the exit hold", name, contract.UnresolvedItemExit)
	}
	if !strings.Contains(refused.errw, item) {
		t.Errorf("the refusal names no item; wanted %s in:\n%s", item, refused.errw)
	}

	if got := runCLI(t, root, "verify", "fx-1/criteria/1", "--text", "the release notes are written, read against the fixture"); got.code != 0 {
		t.Fatalf("verify: %d %s", got.code, got.errw)
	}
	reached := runCLI(t, root, "move", "fx-1", "waiting")
	if reached.code == 0 {
		t.Fatal("a held card reached a column where no owner takes work up")
	}
	if name := refusalNameOf(reached.errw); name != contract.TakesNoWork {
		t.Errorf("with the item settled the refusal is %s, wanted %s, which is the later row the earlier one was standing in front of", name, contract.TakesNoWork)
	}
}

// pullOrderingDefinition holds a card on the way out of the station it stands
// at and reserves the claim at the station beyond it to the operator. Those
// are canLand's rows 12 and 13 after this card's renumbering, and a pull by
// somebody who is not the operator fails both at once.
const pullOrderingDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Pull ordering",
  "columns": [
    { "id": "f00000000001", "title": "Intake", "kind": "intake",
      "instructions": "Intake instructions.\n" },
    { "id": "f00000000002", "title": "Doing", "kind": "work",
      "gate_items": "out", "instructions": "Doing instructions.\n" },
    { "id": "f00000000003", "title": "Review", "kind": "work",
      "operator_owned": true, "instructions": "Review instructions.\n" },
    { "id": "f00000000004", "title": "Done", "kind": "done",
      "instructions": "Done instructions.\n" }
  ]
}`

// TestTheExitHoldAnswersAheadOfTheOperatorReservedClaimOnAPull is dinah-484
// AC-16's run half. Pull reuses canLand, so the row this card inserts moves
// pull's own list as well as the move's, and the insertion is what puts the
// departure's hold ahead of the destination's operator-owned reservation.
//
// The reservation is declared on the destination rather than on the departure,
// because a departure reserved to the operator is refused by pull's own row 6
// inside canRoute, which runs before canLand is reached at all. Row 13 is the
// reservation read at the column the card would land in and be claimed at,
// which is the row this ordering is about.
func TestTheExitHoldAnswersAheadOfTheOperatorReservedClaimOnAPull(t *testing.T) {
	root := newBenchFromDefinition(t, pullOrderingDefinition)
	if got := runCLI(t, root, "add", "Write the release notes"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "doing"); got.code != 0 {
		t.Fatalf("move to doing: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "file", "--column", "doing", "fx-1", "acceptance_criterion", "Somebody has to settle this."); got.code != 0 {
		t.Fatalf("file: %d %s", got.code, got.errw)
	}
	item := soleItemID(t, root, "fx-1")
	t.Setenv("DINAH_ACTOR", "sam")

	refused := runCLI(t, root, "pull", "review")
	if refused.code == 0 {
		t.Fatal("a pull failing both the exit hold and the destination's reservation succeeded")
	}
	if name := refusalNameOf(refused.errw); name != contract.UnresolvedItemExit {
		t.Errorf("the refusal name is %s, wanted %s, so row 13 answered ahead of row 12", name, contract.UnresolvedItemExit)
	}
	if !strings.Contains(refused.errw, item) {
		t.Errorf("the refusal names no item; wanted %s in:\n%s", item, refused.errw)
	}
}

// entryHoldAsWrittenBefore is a workbench carrying the declaration this card
// found on disk and did not touch: the boolean the two-word vocabulary wrote,
// with no direction word anywhere in it.
const entryHoldAsWrittenBefore = `{
  "profile": "dinah-core/0.12",
  "title": "Entry hold",
  "columns": [
    { "id": "f00000000001", "title": "Intake", "kind": "intake",
      "instructions": "Intake instructions.\n" },
    { "id": "f00000000002", "title": "Doing", "kind": "work",
      "gate_items": true, "instructions": "Doing instructions.\n" },
    { "id": "f00000000003", "title": "Review", "kind": "work",
      "instructions": "Review instructions.\n" },
    { "id": "f00000000004", "title": "Done", "kind": "done",
      "instructions": "Done instructions.\n" }
  ]
}`

// TestADeclarationWrittenBeforeTheDirectionBehavesExactlyAsBefore is dinah-484
// AC-10, and it is the one case here run against a workbench rather than
// reasoned about. The declaration is the one the shipped command already
// writes, nothing rewrites it, and the column goes on refusing entry and
// releasing a departure in both directions, which is the whole of what a
// workbench standing on disk today is entitled to.
func TestADeclarationWrittenBeforeTheDirectionBehavesExactlyAsBefore(t *testing.T) {
	root := newBenchFromDefinition(t, entryHoldAsWrittenBefore)
	if got := runCLI(t, root, "add", "Write the release notes"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "file", "--column", "doing", "fx-1", "acceptance_criterion", "Somebody has to settle this."); got.code != 0 {
		t.Fatalf("file: %d %s", got.code, got.errw)
	}
	item := soleItemID(t, root, "fx-1")

	// The declaration is read back off disk exactly as it was written, and
	// the word a person types for it is the word that was typed before this
	// card existed.
	if got := holdOf(t, root, "doing"); got != bench.HoldOn {
		t.Errorf("the declaration reads back as %q, wanted %q", got, bench.HoldOn)
	}
	anchor, _ := columnAnchorOf(t, root, "doing")
	if got := anchor.Value(bench.GateItemsKey); got != "true" {
		t.Errorf("the anchor stores %q under the gate key, wanted the untouched true", got)
	}

	refused := runCLI(t, root, "move", "fx-1", "doing")
	if refused.code == 0 {
		t.Fatal("the move into the column declaring the old flag succeeded")
	}
	if name := refusalNameOf(refused.errw); name != contract.UnresolvedItem {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.UnresolvedItem)
	}
	if !strings.Contains(refused.errw, item) {
		t.Errorf("the refusal names no item; wanted %s in:\n%s", item, refused.errw)
	}

	// A card already standing there leaves in both directions, which is what
	// says the new direction was not switched on for a declaration that never
	// asked for it.
	if got := runCLI(t, root, "move", "fx-1", "doing", "--override"); got.code != 0 {
		t.Fatalf("the operator could not carry the card in: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "review"); got.code != 0 {
		t.Fatalf("the forward departure was held: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "doing", "--override"); got.code != 0 {
		t.Fatalf("the operator could not carry the card back in: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "intake"); got.code != 0 {
		t.Fatalf("the regressive departure was held: %d %s", got.code, got.errw)
	}
}

// TestEveryHoldDirectionIsTypedAndReadBack is dinah-484 AC-2. Each of the four
// words goes in through the command a person types, reaches disk in the
// spelling the format stores, and comes back as the word that was typed.
// Writing the same word twice is an ordinary success, which is what a script
// re-running a declaration depends on.
func TestEveryHoldDirectionIsTypedAndReadBack(t *testing.T) {
	root := newBench(t)
	for _, want := range []struct {
		typed  string
		stored string
	}{
		{bench.HoldOn, "true"},
		{bench.HoldOut, "out"},
		{bench.HoldBoth, "both"},
		{bench.HoldOff, ""},
	} {
		if got := runCLI(t, root, "set", "doing", bench.HoldField, want.typed); got.code != 0 {
			t.Fatalf("set doing hold %s: %d %s", want.typed, got.code, got.errw)
		}
		if got := runCLI(t, root, "set", "doing", bench.HoldField, want.typed); got.code != 0 {
			t.Fatalf("set doing hold %s a second time: %d %s", want.typed, got.code, got.errw)
		}
		anchor, _ := columnAnchorOf(t, root, "doing")
		if got := anchor.Value(bench.GateItemsKey); got != want.stored {
			t.Errorf("the anchor stores %q under the gate key after %s was typed, wanted %q", got, want.typed, want.stored)
		}
		if want.typed == bench.HoldOff && anchor.Has(bench.GateItemsKey) {
			t.Error("turning the hold off left the gate key standing rather than clearing it")
		}
		if got := holdOf(t, root, "doing"); got != want.typed {
			t.Errorf("the hold reads %q after %s was typed, wanted the word that was typed", got, want.typed)
		}
	}

	// The retired word, refused beside the rest. On is the sole spelling for
	// the entry direction, so a reader who guesses the other one is told so.
	refused := runCLI(t, root, "set", "doing", bench.HoldField, "in")
	if name := refusalNameOf(refused.errw); name != contract.Malformed {
		t.Errorf("the word in answered %s, wanted %s", name, contract.Malformed)
	}

	nonOperator := runCLI(t, root, "set", "doing", bench.HoldField, bench.HoldOut, "--actor", "someoneelse")
	if name := refusalNameOf(nonOperator.errw); name != contract.NotOperator {
		t.Errorf("a write by somebody who is not the operator answered %s, wanted %s", name, contract.NotOperator)
	}
}

// itemOwnerOf reads one item's owner key off disk, which is what a case
// asserting a refused write changed nothing has to compare.
func itemOwnerOf(t *testing.T, root, card, item string) string {
	t.Helper()
	text, err := bench.ReadText(itemAnchorPath(t, root, card, item))
	if err != nil {
		t.Fatalf("read the item: %v", err)
	}
	fm, _ := bench.ParseAnchor(text)
	return fm.Value(bench.ItemOwnerField)
}

// operatorOwnedItem files one item of a kind in the operator's name and
// answers the workbench and the item's identifier.
func operatorOwnedItem(t *testing.T, kind string) (string, string) {
	t.Helper()
	root := newBench(t)
	if got := runCLI(t, root, "add", "Write the release notes"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "file", "--owner", bench.ItemOwnerOperator, "fx-1", kind, "Only the operator rules on this."); got.code != 0 {
		t.Fatalf("file a %s: %d %s", kind, got.code, got.errw)
	}
	return root, soleItemID(t, root, "fx-1")
}

// TestAnOperatorOwnedItemIsSettledByTheOperatorAlone is dinah-484 AC-7, and it
// is the half of the card that turns the hold from a convention into a rule.
// An exit hold cleared by whoever happens to be holding the card is not a
// gate, so each of the three settling verbs is refused to anybody but the
// operator and the item on disk is read back to prove the refused call landed
// nothing.
func TestAnOperatorOwnedItemIsSettledByTheOperatorAlone(t *testing.T) {
	for _, settle := range []struct {
		kind string
		verb string
		ref  string
		note string
		land string
	}{
		{"open_question", "resolve", "fx-1/questions/1", "the operator ruled on 2026-09-11", bench.ItemResolved},
		{"acceptance_criterion", "verify", "fx-1/criteria/1", "the operator read it and it holds", bench.ItemVerified},
		{"acceptance_criterion", "fail", "fx-1/criteria/1", "the operator read it and it does not hold", bench.ItemFailed},
	} {
		t.Run(settle.verb, func(t *testing.T) {
			root, item := operatorOwnedItem(t, settle.kind)
			before, err := os.ReadFile(itemAnchorPath(t, root, "fx-1", item))
			if err != nil {
				t.Fatalf("read the item: %v", err)
			}

			refused := runCLI(t, root, settle.verb, settle.ref, "--text", settle.note, "--actor", "sam")
			if refused.code != contract.ExitCode(contract.OutcomeRefused) {
				t.Fatalf("%s by somebody who is not the operator exited %d", settle.verb, refused.code)
			}
			if name := refusalNameOf(refused.errw); name != contract.NotOperator {
				t.Errorf("the refusal name is %s, wanted %s", name, contract.NotOperator)
			}
			after, err := os.ReadFile(itemAnchorPath(t, root, "fx-1", item))
			if err != nil {
				t.Fatalf("reread the item: %v", err)
			}
			if string(after) != string(before) {
				t.Errorf("the refused %s rewrote the item:\n%s\nwanted:\n%s", settle.verb, after, before)
			}

			if got := runCLI(t, root, settle.verb, settle.ref, "--text", settle.note); got.code != 0 {
				t.Fatalf("the operator's own %s: %d %s", settle.verb, got.code, got.errw)
			}
			if state := soleItemState(t, root, "fx-1"); state != settle.land {
				t.Errorf("the item stands at %q after the operator's %s, wanted %q", state, settle.verb, settle.land)
			}
		})
	}
}

// TestAnOperatorOwnedItemIsReopenedByAnybody is dinah-484 AC-8, and it is the
// deliberate hole in the rule above. Reopening returns a closed item to
// pending, which can only re-impose a hold and never lift one, so restricting
// it would protect nothing the operator's ownership needs protected.
func TestAnOperatorOwnedItemIsReopenedByAnybody(t *testing.T) {
	root, _ := operatorOwnedItem(t, "open_question")
	if got := runCLI(t, root, "resolve", "fx-1/questions/1", "--text", "the operator ruled on 2026-09-11"); got.code != 0 {
		t.Fatalf("the operator's own resolve: %d %s", got.code, got.errw)
	}
	if state := soleItemState(t, root, "fx-1"); state != bench.ItemResolved {
		t.Fatalf("the item stands at %q, so this case is not exercising a closed item", state)
	}

	reopened := runCLI(t, root, "reopen", "fx-1/questions/1", "the ruling was recorded against the wrong card", "--actor", "sam")
	if reopened.code != 0 {
		t.Fatalf("a reopen by somebody who is not the operator: %d %s", reopened.code, reopened.errw)
	}
	if state := soleItemState(t, root, "fx-1"); state != bench.ItemPending {
		t.Errorf("the item stands at %q after the reopen, wanted %q", state, bench.ItemPending)
	}
}

// TestAnOperatorOwnedItemsOwnerIsTheOperatorsToRewrite is dinah-484 AC-9, and
// it closes the write-around without which the refusal above is a refusal any
// value satisfies: whoever wanted past it would take the item out of the
// operator's name first and then settle it.
//
// Filing a fresh item in the operator's name stays open to everybody, because
// routing a question to the operator is the ordinary act the field exists for,
// and closing that direction would leave nobody able to ask him anything.
func TestAnOperatorOwnedItemsOwnerIsTheOperatorsToRewrite(t *testing.T) {
	root, item := operatorOwnedItem(t, "open_question")
	path := itemAnchorPath(t, root, "fx-1", item)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the item: %v", err)
	}

	for _, argv := range [][]string{
		{"set", "fx-1/questions/1", bench.ItemOwnerField, "sam"},
		{"set", "fx-1/questions/1", bench.ItemOwnerField},
	} {
		refused := runCLI(t, root, append(argv, "--actor", "sam")...)
		if refused.code != contract.ExitCode(contract.OutcomeRefused) {
			t.Fatalf("%v by somebody who is not the operator exited %d: %s", argv[2:], refused.code, refused.errw)
		}
		if name := refusalNameOf(refused.errw); name != contract.NotOperator {
			t.Errorf("%v answered %s, wanted %s", argv[2:], name, contract.NotOperator)
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reread the item: %v", err)
		}
		if string(after) != string(before) {
			t.Errorf("the refused write rewrote the item:\n%s\nwanted:\n%s", after, before)
		}
	}

	if got := runCLI(t, root, "set", "fx-1/questions/1", bench.ItemOwnerField, "sam"); got.code != 0 {
		t.Fatalf("the operator's own write: %d %s", got.code, got.errw)
	}
	if owner := itemOwnerOf(t, root, "fx-1", item); owner != "sam" {
		t.Errorf("the item records the owner %q after the operator's write, wanted sam", owner)
	}

	// And the way in stays open: somebody who is not the operator routes a
	// fresh question to him, and is then refused when they try to write his
	// name onto an item that does not already carry it.
	if got := runCLI(t, root, "file", "--owner", bench.ItemOwnerOperator, "fx-1", "open_question", "Another one for the operator.", "--actor", "sam"); got.code != 0 {
		t.Fatalf("a non-operator could not file in the operator's name: %d %s", got.code, got.errw)
	}
	claimed := runCLI(t, root, "set", "fx-1/questions/1", bench.ItemOwnerField, bench.ItemOwnerOperator, "--actor", "sam")
	if claimed.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("a non-operator wrote the operator's name onto an existing item: %d %s", claimed.code, claimed.errw)
	}
}

// TestTheOperatorRefusalIsDefeatedByTheActorFlag is dinah-495 AC-3. The
// refusal above is real and this test does not weaken it: what it establishes
// is what the refusal costs a caller who wants past it, which is one flag.
//
// The ladder resolves the actor from the flag first, then from DINAH_ACTOR,
// then from the user's config, so a shell standing as somebody who is not the
// operator still yields the operator on an invocation that names him. Both
// halves run against one workbench and one item, because a test that only
// showed the refusal would pass against a build refusing everything and a test
// that only showed the flag landing would pass against a build refusing
// nothing.
func TestTheOperatorRefusalIsDefeatedByTheActorFlag(t *testing.T) {
	root, _ := operatorOwnedItem(t, "open_question")
	t.Setenv("DINAH_ACTOR", "bo")

	refused := runCLI(t, root, "resolve", "fx-1/questions/1", "--text", "answered")
	if refused.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("resolve as bo exited %d, wanted the refusal: %s", refused.code, refused.errw)
	}
	if name := refusalNameOf(refused.errw); name != contract.NotOperator {
		t.Fatalf("the refusal name is %s, wanted %s", name, contract.NotOperator)
	}
	if state := soleItemState(t, root, "fx-1"); state != bench.ItemPending {
		t.Fatalf("the refused resolve left the item at %q, so the refusal landed something", state)
	}

	landed := runCLI(t, root, "resolve", "fx-1/questions/1", "--text", "answered", "--actor", "alka")
	if landed.code != 0 {
		t.Fatalf("the same invocation naming the operator on the flag: %d %s", landed.code, landed.errw)
	}
	if state := soleItemState(t, root, "fx-1"); state != bench.ItemResolved {
		t.Errorf("the item stands at %q after the flag named the operator, wanted %q", state, bench.ItemResolved)
	}
}

// TestTheOperatorRefusalIsDefeatedByWritingTheFile is dinah-495 AC-4, and it
// is the defeat that needs no flag. A workbench is files on disk, an item's
// state is a line of frontmatter, and dinah path hands the caller the file, so
// whoever holds the filesystem settles an operator-owned item with an editor.
//
// The test asserts what the tool says afterwards as well as what the file
// says, because the point is not that a hand edit is possible but that nothing
// downstream reports it: the item reads as settled, dinah check finds no
// structural defect, and the journal carries no event saying it was settled.
func TestTheOperatorRefusalIsDefeatedByWritingTheFile(t *testing.T) {
	root, item := operatorOwnedItem(t, "open_question")
	path := itemAnchorPath(t, root, "fx-1", item)

	printed := runCLI(t, root, "path", "fx-1/questions/1")
	if printed.code != 0 {
		t.Fatalf("path: %d %s", printed.code, printed.errw)
	}
	// os.SameFile rather than a string comparison, because the two spellings
	// are allowed to differ: on macOS the temporary directory reaches the
	// test as /var/... and the tool answers with the resolved /private/var/...
	// What the assertion is about is that path hands over this item's own
	// anchor, and SameFile is the documented way to ask that.
	printedInfo, err := os.Stat(strings.TrimSpace(printed.out))
	if err != nil {
		t.Fatalf("stat what path printed: %v", err)
	}
	anchorInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat the item's anchor: %v", err)
	}
	if !os.SameFile(printedInfo, anchorInfo) {
		t.Fatalf("path printed %q, which is not the item's own anchor %q", strings.TrimSpace(printed.out), path)
	}

	before, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read the item: %v", err)
	}
	pending := bench.ItemStateField + ": " + bench.ItemPending
	if !strings.Contains(before, pending) {
		t.Fatalf("the anchor carries no %q line, so this test is not editing what it thinks it is:\n%s", pending, before)
	}
	edited := strings.Replace(before, pending, bench.ItemStateField+": "+bench.ItemResolved, 1)
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatalf("write the item: %v", err)
	}

	if state := soleItemState(t, root, "fx-1"); state != bench.ItemResolved {
		t.Fatalf("the edited item reads %q, wanted %q", state, bench.ItemResolved)
	}
	shown := runCLI(t, root, "show", "fx-1", "--fields", "checklist")
	if shown.code != 0 {
		t.Fatalf("show: %d %s", shown.code, shown.errw)
	}
	if !strings.Contains(shown.out, bench.ItemResolved) {
		t.Errorf("show does not report the item resolved:\n%s", shown.out)
	}
	checked := runCLI(t, root, "check")
	if checked.code != 0 {
		t.Errorf("check exited %d after the hand edit, wanted 0: %s %s", checked.code, checked.out, checked.errw)
	}

	journal := filepath.Join(soleBenchDir(t, root), bench.CardsDir, cardID(t, root, "fx-1"), bench.JournalName)
	events, _, err := bench.ReadJournal(journal)
	if err != nil {
		t.Fatalf("read the journal: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("the card's journal is empty, so the absence asserted below would hold against a journal nothing ever wrote")
	}
	for _, event := range events {
		if event.Event == contract.EventItemResolved {
			t.Errorf("the journal records %s, so the hand edit was journaled after all", event.Event)
		}
	}
}

// notOperatorLine is the not-operator refusal a caller declaring no harness
// has always read, and notOperatorHarnessLine is the one dinah-563 gives a
// caller that declared the claude-code harness. Both are written out rather
// than composed from the catalog, so a catalog edit nobody approved fails
// here instead of moving the expectation with it.
const (
	notOperatorLine        = "not-operator this action is the operator's, and you are planner; ask the operator to run it, or run `dinah whoami` to see who Dinah takes you to be"
	notOperatorHarnessLine = "not-operator this action is the operator's, and you are planner; you declared the claude-code harness, and this act is the operator's to perform; if the operator has stated a ruling on it, read `dinah guide on-behalf` before you record it"
)

// onBehalfDefinition reserves its Review station to the operator, so one
// workbench reaches the claim and the move that reservation refuses.
const onBehalfDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "On behalf",
  "columns": [
    { "id": "f00000000001", "title": "Intake", "kind": "intake" },
    { "id": "f00000000002", "title": "Doing", "kind": "work" },
    { "id": "f00000000003", "title": "Review", "kind": "work", "operator_owned": true },
    { "id": "f00000000004", "title": "Done", "kind": "done" }
  ]
}`

// TestANotOperatorRefusalPointsAHarnessAtTheGuide is dinah-563's refusal
// half. A caller that declared a harness is told the act is the operator's
// and pointed at the guide on recording a ruling the operator stated, and a
// caller that declared none reads the line it always read, byte for byte, so
// the quick start's transcript of a person meeting the refusal stands
// unchanged. Both readers are asserted on one item, because a build printing
// either line to everybody would pass a test that looked at one of them.
//
// The harness line is then asserted at four more raise sites, which between
// them reach both routes the value takes: unblock, the move and the claim
// through (*Library).refuse, and reshape through its own direct RefuseWith.
func TestANotOperatorRefusalPointsAHarnessAtTheGuide(t *testing.T) {
	t.Run("resolve", func(t *testing.T) {
		root, _ := operatorOwnedItem(t, "open_question")
		argv := []string{"resolve", "fx-1/questions/1", "--text", "Garden", "--actor", "planner"}

		t.Setenv("DINAH_HARNESS", "claude-code")
		if got := runCLI(t, root, argv...); got.errw != notOperatorHarnessLine+"\n" {
			t.Errorf("a resolve declaring a harness printed:\n%q\nwanted:\n%q", got.errw, notOperatorHarnessLine+"\n")
		}
		// refusalContextOf is collection_reference_test.go's reader of the
		// machine form, which answers a nil map when no context member came.
		if name, context := refusalContextOf(t, root, argv...); name != contract.NotOperator || context["harness"] != "claude-code" || len(context) != 1 {
			t.Errorf("the machine form of a harness-declaring refusal is %s with the context %v, wanted %s with harness claude-code alone", name, context, contract.NotOperator)
		}

		t.Setenv("DINAH_HARNESS", "")
		if got := runCLI(t, root, argv...); got.errw != notOperatorLine+"\n" {
			t.Errorf("a resolve declaring no harness printed:\n%q\nwanted today's line:\n%q", got.errw, notOperatorLine+"\n")
		}
		if name, context := refusalContextOf(t, root, argv...); name != contract.NotOperator || context != nil {
			t.Errorf("the machine form of a refusal declaring no harness is %s with the context %v, wanted %s with none", name, context, contract.NotOperator)
		}
		if state := soleItemState(t, root, "fx-1"); state != bench.ItemPending {
			t.Errorf("the refused resolves left the item at %q", state)
		}
	})

	root := newBenchFromDefinition(t, onBehalfDefinition)
	if got := runCLI(t, root, "add", "Choose the venue"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "review"); got.code != 0 {
		t.Fatalf("move to review: %d %s", got.code, got.errw)
	}
	source := reshapeSourceFrom(t, root, func(columns []map[string]any) []map[string]any { return columns })
	t.Setenv("DINAH_HARNESS", "claude-code")
	for _, act := range [][]string{
		{"unblock", "fx-1"},
		{"move", "fx-1", "done"},
		{"claim", "fx-1"},
		{"reshape", "--from", source, "--yes"},
	} {
		t.Run(act[0], func(t *testing.T) {
			got := runCLI(t, root, append(act, "--actor", "planner")...)
			if got.errw != notOperatorHarnessLine+"\n" {
				t.Errorf("%v declaring a harness printed:\n%q\nwanted:\n%q", act, got.errw, notOperatorHarnessLine+"\n")
			}
		})
	}
}

// TestAnActRecordedForTheOperatorShowsItsPerformerOnlyInTheMachineForm proves
// the sentence guidepin.TheNameAloneShowsAtATerminal pins in the on-behalf
// guide. An agent records a ruling under the operator's name with its
// harness, provider and model declared. The machine form of the journal
// carries all four facts, and the plain journal listing, `dinah changes` and
// the resolution comment's author line carry the name alone. dinah-564 is the
// card that makes the plain surfaces show the performer, and when it lands
// this test and the pinned sentence change together.
//
// Every variable the identity ladder reads is set explicitly before each
// invocation, so the inherited environment cannot answer for any of them.
func TestAnActRecordedForTheOperatorShowsItsPerformerOnlyInTheMachineForm(t *testing.T) {
	root, _ := operatorOwnedItem(t, "open_question")
	declared := []string{"claude-code", "anthropic", "claude-opus-5"}
	asPerson := func() {
		t.Setenv("DINAH_ACTOR", "alka")
		t.Setenv("DINAH_HARNESS", "")
		t.Setenv("DINAH_PROVIDER", "")
		t.Setenv("DINAH_MODEL", "")
	}

	asPerson()
	minted := runCLI(t, root, "changes")
	cursor := ""
	for _, line := range strings.Split(minted.out, "\n") {
		if rest, found := strings.CutPrefix(line, "cursor: "); found {
			cursor = rest
		}
	}
	if minted.code != 0 || cursor == "" {
		t.Fatalf("changes minted no cursor: %d %q %s", minted.code, minted.out, minted.errw)
	}

	t.Setenv("DINAH_ACTOR", "planner")
	t.Setenv("DINAH_HARNESS", declared[0])
	t.Setenv("DINAH_PROVIDER", declared[1])
	t.Setenv("DINAH_MODEL", declared[2])
	note := "Alka, in this conversation: Take the garden. Recorded for her by planner."
	if got := runCLI(t, root, "--actor", "alka", "resolve", "fx-1/questions/1", "--text", note); got.code != 0 {
		t.Fatalf("the resolve recorded for the operator: %d %s", got.code, got.errw)
	}

	asPerson()
	machine := runCLI(t, root, "--json", "list", "fx-1/journal")
	var events []map[string]any
	if err := json.Unmarshal([]byte(machine.out), &events); err != nil {
		t.Fatalf("read the journal's machine form: %v\n%s", err, machine.out)
	}
	resolved := 0
	for _, event := range events {
		if event["event"] != contract.EventItemResolved {
			continue
		}
		resolved++
		actor, _ := event["actor"].(map[string]any)
		want := map[string]any{"name": "alka", "harness": declared[0], "gen_ai.provider.name": declared[1], "gen_ai.request.model": declared[2]}
		for key, value := range want {
			if actor[key] != value {
				t.Errorf("the machine form's item_resolved actor carries %s=%v, wanted %v: %v", key, actor[key], value, actor)
			}
		}
	}
	if resolved != 1 {
		t.Fatalf("the machine form carries %d item_resolved entries, wanted one", resolved)
	}

	// nameAloneOn finds the one row of a plain listing that names the event,
	// and holds it to the operator's name in its Actor cell and to none of the
	// three declared values anywhere on the row.
	nameAloneOn := func(surface, listing string, actorCell int) {
		t.Helper()
		var rows []string
		for _, line := range strings.Split(listing, "\n") {
			if strings.Contains(line, contract.EventItemResolved) {
				rows = append(rows, line)
			}
		}
		if len(rows) != 1 {
			t.Fatalf("%s carries %d rows naming %s, wanted one:\n%s", surface, len(rows), contract.EventItemResolved, listing)
		}
		cells := strings.Fields(rows[0])
		if len(cells) <= actorCell || cells[actorCell] != "alka" {
			t.Errorf("%s's row does not carry alka in its Actor cell: %q", surface, rows[0])
		}
		for _, value := range declared {
			if strings.Contains(rows[0], value) {
				t.Errorf("%s's row carries the declared %s, so the guide's sentence is no longer true: %q", surface, value, rows[0])
			}
		}
	}
	listed := runCLI(t, root, "list", "fx-1/journal")
	if listed.code != 0 {
		t.Fatalf("list the journal: %d %s", listed.code, listed.errw)
	}
	nameAloneOn("the plain journal listing", listed.out, 2)
	changed := runCLI(t, root, "changes", "--since", cursor)
	if changed.code != 0 {
		t.Fatalf("changes: %d %s", changed.code, changed.errw)
	}
	nameAloneOn("dinah changes", changed.out, 3)

	shown := runCLI(t, root, "show", "fx-1/questions/1")
	if shown.code != 0 {
		t.Fatalf("show the item: %d %s", shown.code, shown.errw)
	}
	var who [][]string
	for _, line := range strings.Split(shown.out, "\n") {
		if cells := strings.Fields(line); len(cells) > 0 && cells[0] == "Who" {
			who = append(who, cells)
		}
	}
	if len(who) != 1 || len(who[0]) != 2 || who[0][1] != "alka" {
		t.Errorf("the resolution comment's author line is %v, wanted Who and the name alone", who)
	}
}
