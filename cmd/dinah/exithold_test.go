package main

import (
	"os"
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

// TestAColumnHoldingOnTheWayOutRefusesTheDeparture is dinah-484 AC-4 and AC-5.
// The forward departure and the regressive one are both refused while the item
// stands unsettled, because nothing in the check reads the direction of the
// move: a push-back carrying an unanswered question is exactly what the hold
// exists to stop.
//
// The item is then settled and the same forward move goes through, which is
// what stops this passing on a build that refuses every move out of the
// station whatever the card carries.
func TestAColumnHoldingOnTheWayOutRefusesTheDeparture(t *testing.T) {
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
	if back.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("the regressive departure exited %d, wanted %d", back.code, contract.ExitCode(contract.OutcomeRefused))
	}
	if name := refusalNameOf(back.errw); name != contract.UnresolvedItemExit {
		t.Errorf("the regressive departure answered %s, wanted %s, so the hold reads the direction of the move", name, contract.UnresolvedItemExit)
	}
	if !strings.Contains(back.errw, item) {
		t.Errorf("the refusal names no item; wanted %s in:\n%s", item, back.errw)
	}

	if got := runCLI(t, root, "verify", "fx-1/criteria/1", "the release notes are written, read against the fixture"); got.code != 0 {
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

	if got := runCLI(t, root, "verify", "fx-1/criteria/1", "the release notes are written, read against the fixture"); got.code != 0 {
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

			refused := runCLI(t, root, settle.verb, settle.ref, settle.note, "--actor", "sam")
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

			if got := runCLI(t, root, settle.verb, settle.ref, settle.note); got.code != 0 {
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
	if got := runCLI(t, root, "resolve", "fx-1/questions/1", "the operator ruled on 2026-09-11"); got.code != 0 {
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
