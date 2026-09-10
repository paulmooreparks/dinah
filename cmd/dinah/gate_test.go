package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// declareGateItems writes gate_items into one column's own anchor, found by
// the title the init flow gives it. Nothing in the tool sets the field, so a
// test reaching the rendered surface writes the declaration the way a person
// editing a column.md would, which is what declareLoopLimit already does one
// file over for the other column-level declaration.
func declareGateItems(t *testing.T, root, title, value string) {
	t.Helper()
	columns := filepath.Join(soleBenchDir(t, root), bench.ColumnsDir)
	for _, id := range bench.ListIDs(columns) {
		path := filepath.Join(columns, id, bench.ColumnAnchor)
		text, err := bench.ReadText(path)
		if err != nil {
			t.Fatalf("read a column: %v", err)
		}
		fm, body := bench.ParseAnchor(text)
		if fm.Value("title") != title {
			continue
		}
		fm.Set("gate_items", value)
		if err := os.WriteFile(path, []byte(fm.Render(body)), 0o644); err != nil {
			t.Fatalf("write a column: %v", err)
		}
		return
	}
	t.Fatalf("the fixture flow carries no column titled %s", title)
}

// soleItemID names the one checklist item a fixture card carries. The refusal
// names the item by identifier, and the identifier is minted at file time, so
// a test asserting what the refusal names has to read it back rather than
// spelling it.
func soleItemID(t *testing.T, root, card string) string {
	t.Helper()
	collection := filepath.Join(soleBenchDir(t, root), bench.CardsDir, cardID(t, root, card), bench.ChecklistDir)
	ids := bench.ListIDs(collection)
	if len(ids) != 1 {
		t.Fatalf("wanted one item on %s, got %v", card, ids)
	}
	return ids[0]
}

// gatedCard builds a workbench whose doing station holds a card, files one
// item of the given kind against that station, and answers the item's
// identifier. The card is standing at intake when this returns, which is one
// legal move away from the held station.
func gatedCard(t *testing.T, kind string) (string, string) {
	t.Helper()
	root := newBench(t)
	declareGateItems(t, root, "Doing", "true")
	if got := runCLI(t, root, "add", "Write the release notes"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	held := columnIdentifier(t, root, "doing")
	if got := runCLI(t, root, "file", "--column", held, "fx-1", kind, "Somebody has to settle this."); got.code != 0 {
		t.Fatalf("file a %s: %d %s", kind, got.code, got.errw)
	}
	return root, soleItemID(t, root, "fx-1")
}

// TestAGatedColumnHoldsEveryKindOnItsOwn is dinah-450 AC-5's first row and the
// run half of AC-8. CORE-GATE-1 puts the selectivity in which items name a
// column rather than in the column or in the tool, so each kind is driven on
// its own fixture: a build holding two kinds and letting the third through
// would pass a case that filed all three and counted one refusal.
//
// The kind that matters most here is the acceptance criterion, which the claim
// refusal exempts unconditionally under CORE-CLAIM-9. A build that reused that
// exemption would let this move through.
func TestAGatedColumnHoldsEveryKindOnItsOwn(t *testing.T) {
	for _, kind := range bench.ItemKinds {
		t.Run(kind, func(t *testing.T) {
			root, item := gatedCard(t, kind)

			got := runCLI(t, root, "move", "fx-1", "doing")
			if got.code == 0 {
				t.Fatalf("the move into the held station succeeded carrying a pending %s", kind)
			}
			if !strings.Contains(got.errw, contract.UnresolvedItem) {
				t.Errorf("wanted %s, got:\n%s", contract.UnresolvedItem, got.errw)
			}
			if !strings.Contains(got.errw, item) {
				t.Errorf("the refusal names no item; wanted %s in:\n%s", item, got.errw)
			}
		})
	}
}

// TestAnItemSettledAtAGatedColumnHoldsNothing is dinah-450 AC-5's last row,
// and it is what stops the test above passing on a build that refuses every
// move into the station whatever the card carries.
func TestAnItemSettledAtAGatedColumnHoldsNothing(t *testing.T) {
	root, _ := gatedCard(t, "acceptance_criterion")
	if got := runCLI(t, root, "verify", "fx-1/criteria/1", "the endpoint answers 404, run against the fixture"); got.code != 0 {
		t.Fatalf("verify: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "doing"); got.code != 0 {
		t.Fatalf("the move was refused with the item settled: %d %s", got.code, got.errw)
	}
}

// TestAColumnDeclaringNoGateHoldsNothing is the negative control for the whole
// mechanism. Without it, a build refusing a move into any column carrying an
// unresolved item would pass every assertion above.
func TestAColumnDeclaringNoGateHoldsNothing(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "Write the release notes"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	held := columnIdentifier(t, root, "doing")
	if got := runCLI(t, root, "file", "--column", held, "fx-1", "acceptance_criterion", "Somebody has to settle this."); got.code != 0 {
		t.Fatalf("file: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "doing"); got.code != 0 {
		t.Fatalf("a column declaring no gate refused the move: %d %s", got.code, got.errw)
	}
}

// TestTheOperatorCarriesACardPastAGate is dinah-450 AC-5's second row and
// CORE-GATE-4. The marker is the operator's, it carries the one move it is
// passed on, and the act is recorded as an override rather than passing as an
// ordinary move.
func TestTheOperatorCarriesACardPastAGate(t *testing.T) {
	root, _ := gatedCard(t, "open_question")

	if got := runCLI(t, root, "move", "fx-1", "doing", "--override"); got.code != 0 {
		t.Fatalf("the operator's override was refused: %d %s", got.code, got.errw)
	}
	events := cardJournal(t, root, cardID(t, root, "fx-1"))
	var moves int
	for _, ev := range events {
		if ev.Event != contract.EventMoved {
			continue
		}
		moves++
		if !ev.Override {
			t.Errorf("the moved event carries no override flag: %+v", ev)
		}
	}
	if moves != 1 {
		t.Fatalf("wanted one moved event, got %d", moves)
	}
}

// TestANonOperatorIsRefusedTheOverrideAtAGate is dinah-450 AC-5's third row.
// It proves no new override path was opened: the marker is refused by the row
// canRoute already carries, ahead of the gate, so the answer is not-operator
// rather than unresolved-item.
func TestANonOperatorIsRefusedTheOverrideAtAGate(t *testing.T) {
	root, _ := gatedCard(t, "decision")
	t.Setenv("DINAH_ACTOR", "sam")

	got := runCLI(t, root, "move", "fx-1", "doing", "--override")
	if got.code == 0 {
		t.Fatal("an owner who is not the operator carried a card past the gate")
	}
	if !strings.Contains(got.errw, contract.NotOperator) {
		t.Errorf("wanted %s, got:\n%s", contract.NotOperator, got.errw)
	}
}

// TestAPullIsHeldByTheDestinationsGate is dinah-450 AC-6's pull row driven
// rather than only listed. A pull lands a card in the column it names, so
// CORE-GATE-3 reaches it the same way a move reaches it, and pullChecks states
// the row for real rather than inheriting it silently.
func TestAPullIsHeldByTheDestinationsGate(t *testing.T) {
	root, item := gatedCard(t, "acceptance_criterion")

	got := runCLI(t, root, "pull", "doing")
	if got.code == 0 {
		t.Fatal("the pull into the held station succeeded")
	}
	if !strings.Contains(got.errw, contract.UnresolvedItem) {
		t.Errorf("wanted %s, got:\n%s", contract.UnresolvedItem, got.errw)
	}
	if !strings.Contains(got.errw, item) {
		t.Errorf("the refusal names no item; wanted %s in:\n%s", item, got.errw)
	}
}

// soleItemState reads back the state of the one checklist item a fixture card
// carries. A test asserting that a failed criterion still holds has to prove
// the criterion is actually failed, because a build where the fail verb landed
// nothing would otherwise pass on a pending item.
func soleItemState(t *testing.T, root, card string) string {
	t.Helper()
	path := filepath.Join(soleBenchDir(t, root), bench.CardsDir, cardID(t, root, card),
		bench.ChecklistDir, soleItemID(t, root, card), bench.ItemAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read the item: %v", err)
	}
	fm, _ := bench.ParseAnchor(text)
	return fm.Value(bench.ItemStateField)
}

// TestAFailedCriterionGoesOnHoldingTheGate is the operator's ruling of
// 2026-09-10, recorded as dinah-450 OQ-5. It names the failed state rather
// than testing the hold in general, because the hold fired on a pending item
// before that ruling and goes on firing on one after it: a guard proving only
// that the gate holds at all stays green on the build this test exists to
// refuse.
//
// The state is the whole point. A failed criterion records that somebody
// checked the work and it did not hold, so a build settling the gate on it
// opens the column on the one state saying the work is wrong.
func TestAFailedCriterionGoesOnHoldingTheGate(t *testing.T) {
	root, item := gatedCard(t, "acceptance_criterion")
	if got := runCLI(t, root, "fail", "fx-1/criteria/1", "the endpoint still answers 200 for an unknown id"); got.code != 0 {
		t.Fatalf("fail: %d %s", got.code, got.errw)
	}
	if state := soleItemState(t, root, "fx-1"); state != bench.ItemFailed {
		t.Fatalf("the criterion stands at %q, so this test is not exercising the failed state", state)
	}

	refused := runCLI(t, root, "move", "fx-1", "doing")
	if refused.code == 0 {
		t.Fatalf("the move into the held station succeeded carrying a failed criterion")
	}
	if !strings.Contains(refused.errw, contract.UnresolvedItem) {
		t.Errorf("wanted %s, got:\n%s", contract.UnresolvedItem, refused.errw)
	}
	if !strings.Contains(refused.errw, item) {
		t.Errorf("the refusal names no item; wanted %s in:\n%s", item, refused.errw)
	}

	// The interim route the ruling relies on. No state carries permission to
	// proceed past a criterion that genuinely failed, so the operator's
	// move-level marker is the only way through, and a build where it stopped
	// working would leave such a card stuck at the gate for good.
	if carried := runCLI(t, root, "move", "fx-1", "doing", "--override"); carried.code != 0 {
		t.Fatalf("the operator could not carry the failed criterion past the gate: %d %s", carried.code, carried.errw)
	}
}

// TestAGatedColumnAnswersAheadOfTheDepartureLoopLimit is dinah-450 OQ-2's
// ruling. The gate is published ninth and the loop limit tenth, and dinah help
// move promises the rows in the order each is checked, so a move failing both
// answers the earlier of the two.
//
// The fixture is the only shape that reaches the pair: a regressive move out
// of a column already at its declared loop limit, into a column holding an
// item of the card's. Either row alone refuses the move, so the assertion is
// on which name comes back rather than on whether it was refused.
func TestAGatedColumnAnswersAheadOfTheDepartureLoopLimit(t *testing.T) {
	root := newBench(t)
	declareLoopLimit(t, root, "Doing", "1")
	loopedCard(t, root)
	// Declared after the fixture's own regressive move, which would otherwise
	// be the move this gate refused.
	declareGateItems(t, root, "Intake", "true")
	back := columnIdentifier(t, root, "intake")
	if got := runCLI(t, root, "file", "--column", back, "fx-1", "open_question", "Somebody has to settle this."); got.code != 0 {
		t.Fatalf("file: %d %s", got.code, got.errw)
	}

	refused := runCLI(t, root, "move", "fx-1", "intake")
	if refused.code == 0 {
		t.Fatal("a regressive move failing both the gate and the loop limit succeeded")
	}
	if !strings.Contains(refused.errw, contract.UnresolvedItem) {
		t.Errorf("wanted %s, the ninth row, got:\n%s", contract.UnresolvedItem, refused.errw)
	}
	if strings.Contains(refused.errw, contract.AtLoopLimit) {
		t.Errorf("the tenth row answered ahead of the ninth:\n%s", refused.errw)
	}
}
