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
// the title the init flow gives it. `dinah set <column> hold on|off` writes the
// same key through the field grammar, and dinah-477's own cases below drive
// that command; this helper stays because it reaches values the command
// refuses, and because it writes the declaration the way a person editing a
// column.md would.
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

// columnAnchorOf reads one column's anchor by slug and answers its header and
// its body, so a case asserts on the key a write touched and on every key it
// left alone.
func columnAnchorOf(t *testing.T, root, slug string) (*bench.Frontmatter, string) {
	t.Helper()
	path := filepath.Join(soleBenchDir(t, root), bench.ColumnsDir, columnIdentifier(t, root, slug), bench.ColumnAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read the %s anchor: %v", slug, err)
	}
	fm, body := bench.ParseAnchor(text)
	return fm, body
}

// benchJournalLines counts the lines standing in the workbench's own journal,
// which is where a column's field write lands its event.
func benchJournalLines(t *testing.T, root string) int {
	t.Helper()
	text, err := os.ReadFile(filepath.Join(soleBenchDir(t, root), bench.JournalName))
	if err != nil {
		t.Fatalf("read the workbench journal: %v", err)
	}
	lines := 0
	for _, line := range strings.Split(string(text), "\n") {
		if strings.TrimSpace(line) != "" {
			lines++
		}
	}
	return lines
}

// holdOf reads a column's hold back through the command a person would use.
func holdOf(t *testing.T, root, slug string) string {
	t.Helper()
	read := runCLI(t, root, "get", slug, bench.HoldField)
	if read.code != 0 {
		t.Fatalf("get %s hold: %d %s", slug, read.code, read.errw)
	}
	return strings.TrimSuffix(read.out, "\n")
}

// TestTheHoldWritesTheDeclarationAndReadsItBack is dinah-477 AC-2 and AC-3.
// Turning the hold on writes the stored key and touches nothing else in the
// header, turning it off removes that key rather than storing a second
// spelling of off, and a column that has never carried the key reads off.
func TestTheHoldWritesTheDeclarationAndReadsItBack(t *testing.T) {
	root := newBench(t)
	before, body := columnAnchorOf(t, root, "doing")
	if got := holdOf(t, root, "doing"); got != bench.HoldOff {
		t.Errorf("a column that has never carried the key reads %q, wanted %q", got, bench.HoldOff)
	}

	if got := runCLI(t, root, "set", "doing", bench.HoldField, bench.HoldOn); got.code != 0 {
		t.Fatalf("set doing hold on: %d %s", got.code, got.errw)
	}
	on, onBody := columnAnchorOf(t, root, "doing")
	if on.Value(bench.GateItemsKey) != "true" {
		t.Errorf("the anchor stores %q under the gate key, wanted true", on.Value(bench.GateItemsKey))
	}
	if onBody != body {
		t.Errorf("the write moved the column's body:\n%q\nwanted:\n%q", onBody, body)
	}
	for _, key := range on.Keys() {
		if key == bench.GateItemsKey {
			continue
		}
		if on.Value(key) != before.Value(key) {
			t.Errorf("the write moved %s from %q to %q", key, before.Value(key), on.Value(key))
		}
	}
	for _, key := range before.Keys() {
		if !on.Has(key) {
			t.Errorf("the write dropped the key %s", key)
		}
	}
	if got := holdOf(t, root, "doing"); got != bench.HoldOn {
		t.Errorf("the hold reads %q after being turned on, wanted %q", got, bench.HoldOn)
	}

	if got := runCLI(t, root, "set", "doing", bench.HoldField, bench.HoldOff); got.code != 0 {
		t.Fatalf("set doing hold off: %d %s", got.code, got.errw)
	}
	off, _ := columnAnchorOf(t, root, "doing")
	if off.Has(bench.GateItemsKey) {
		t.Errorf("turning the hold off left the gate key standing at %q, wanted the key gone", off.Value(bench.GateItemsKey))
	}
	if got := holdOf(t, root, "doing"); got != bench.HoldOff {
		t.Errorf("the hold reads %q after being turned off, wanted %q", got, bench.HoldOff)
	}
}

// TestTheHoldWrittenTwiceJournalsOnce is dinah-477 AC-4. The second write
// stores what the column already carries, which writeField answers ok and
// records nowhere, so the journal is what says the two calls were one act.
func TestTheHoldWrittenTwiceJournalsOnce(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "set", "doing", bench.HoldField, bench.HoldOn); got.code != 0 {
		t.Fatalf("the first write: %d %s", got.code, got.errw)
	}
	before := benchJournalLines(t, root)
	if got := runCLI(t, root, "set", "doing", bench.HoldField, bench.HoldOn); got.code != 0 {
		t.Fatalf("the second write: %d %s", got.code, got.errw)
	}
	if after := benchJournalLines(t, root); after != before {
		t.Errorf("the second write appended %d journal lines, wanted none", after-before)
	}
	if got := holdOf(t, root, "doing"); got != bench.HoldOn {
		t.Errorf("the hold reads %q after being turned on twice, wanted %q", got, bench.HoldOn)
	}
}

// TestTheHoldJournalsInTheStoredForm is dinah-477 AC-7. The journal is the one
// surface that keeps the storage spelling, because nothing reads a column's
// journal back to a person and every other field's event carries what the
// anchor carries.
func TestTheHoldJournalsInTheStoredForm(t *testing.T) {
	root := newBench(t)
	path := filepath.Join(soleBenchDir(t, root), bench.JournalName)
	before, torn, err := bench.ReadJournal(path)
	if err != nil || torn {
		t.Fatalf("read the workbench journal: %v (torn %v)", err, torn)
	}
	if got := runCLI(t, root, "set", "doing", bench.HoldField, bench.HoldOn); got.code != 0 {
		t.Fatalf("set doing hold on: %d %s", got.code, got.errw)
	}
	after, torn, err := bench.ReadJournal(path)
	if err != nil || torn {
		t.Fatalf("reread the workbench journal: %v (torn %v)", err, torn)
	}
	if len(after) != len(before)+1 {
		t.Fatalf("the write appended %d events, wanted one", len(after)-len(before))
	}
	event := after[len(after)-1]
	if event.Event != contract.EventColumnUpdated {
		t.Errorf("the event is %s, wanted %s", event.Event, contract.EventColumnUpdated)
	}
	if event.Field != bench.HoldField {
		t.Errorf("the event names the field %q, wanted %q", event.Field, bench.HoldField)
	}
	if event.From != "" || event.To != "true" {
		t.Errorf("the event carries from %q to %q, wanted the stored form: %q to %q", event.From, event.To, "", "true")
	}
}

// TestTheHoldRefusesAnyValueButOnAndOff is dinah-477 AC-5. A value outside the
// two is refused malformed naming the field, and so is a write carrying no
// value at all, since the field declares itself unclearable exactly so that a
// bare write does nothing rather than quietly meaning off.
func TestTheHoldRefusesAnyValueButOnAndOff(t *testing.T) {
	for _, argv := range [][]string{
		{"set", "doing", bench.HoldField},
		{"set", "doing", bench.HoldField, "maybe"},
		{"set", "doing", bench.HoldField, "true"},
		{"set", "doing", bench.HoldField, "On"},
	} {
		t.Run(strings.Join(argv[2:], " "), func(t *testing.T) {
			root := newBench(t)
			before, _ := columnAnchorOf(t, root, "doing")
			refused := runCLI(t, root, argv...)
			if refused.code != contract.ExitCode(contract.OutcomeRefused) {
				t.Fatalf("the write exited %d, wanted %d", refused.code, contract.ExitCode(contract.OutcomeRefused))
			}
			if name := refusalNameOf(refused.errw); name != contract.Malformed {
				t.Errorf("the refusal name is %s, wanted %s", name, contract.Malformed)
			}
			if !strings.Contains(refused.errw, bench.HoldField) {
				t.Errorf("the refusal names no field; wanted %s in:\n%s", bench.HoldField, refused.errw)
			}
			after, _ := columnAnchorOf(t, root, "doing")
			if after.Has(bench.GateItemsKey) {
				t.Errorf("the refused write stored the gate key: %q", after.Value(bench.GateItemsKey))
			}
			if len(after.Keys()) != len(before.Keys()) {
				t.Errorf("the refused write left %d keys, wanted %d", len(after.Keys()), len(before.Keys()))
			}
		})
	}
}

// TestTheHoldIsTheOperatorsToTurn is dinah-477 AC-6. Nothing here is new
// authority: the hold is a column field, every write to a column field is the
// operator's already, and this case is what says the new field inherited that
// rather than stepping around it. Reading stays open to anybody.
func TestTheHoldIsTheOperatorsToTurn(t *testing.T) {
	root := newBench(t)
	refused := runCLI(t, root, "set", "doing", bench.HoldField, bench.HoldOn, "--actor", "someoneelse")
	if refused.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("a write by somebody who is not the operator exited %d", refused.code)
	}
	if name := refusalNameOf(refused.errw); name != contract.NotOperator {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.NotOperator)
	}
	if anchor, _ := columnAnchorOf(t, root, "doing"); anchor.Has(bench.GateItemsKey) {
		t.Error("the refused write stored the declaration anyway")
	}
	if got := runCLI(t, root, "set", "doing", bench.HoldField, bench.HoldOn); got.code != 0 {
		t.Fatalf("the operator's own write: %d %s", got.code, got.errw)
	}
	read := runCLI(t, root, "get", "doing", bench.HoldField, "--actor", "someoneelse")
	if read.code != 0 {
		t.Fatalf("a read by somebody who is not the operator: %d %s", read.code, read.errw)
	}
	if got := strings.TrimSuffix(read.out, "\n"); got != bench.HoldOn {
		t.Errorf("the read answered %q, wanted %q", got, bench.HoldOn)
	}
}

// TestTheHoldCommandTurnsTheGateOnAndOff is dinah-477 AC-11, and it is the one
// case here that shows the command doing something rather than storing
// something. Every other case above proves a value reached disk and came back;
// this one carries a card at a column the command has just put a hold on,
// watches the move refuse, takes the hold off through the same command, and
// watches the same move go through.
func TestTheHoldCommandTurnsTheGateOnAndOff(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "Write the release notes"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	held := columnIdentifier(t, root, "doing")
	// An acceptance criterion rather than an open question. An unresolved
	// question refuses a claim wherever the card stands, so a case built on
	// one cannot tell the column's own hold from that refusal.
	if got := runCLI(t, root, "file", "--column", held, "fx-1", "acceptance_criterion", "Somebody has to settle this."); got.code != 0 {
		t.Fatalf("file: %d %s", got.code, got.errw)
	}
	item := soleItemID(t, root, "fx-1")

	if got := runCLI(t, root, "move", "fx-1", "doing"); got.code != 0 {
		t.Fatalf("the move was refused before any hold was turned on: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "intake"); got.code != 0 {
		t.Fatalf("the move back to intake: %d %s", got.code, got.errw)
	}

	if got := runCLI(t, root, "set", "doing", bench.HoldField, bench.HoldOn); got.code != 0 {
		t.Fatalf("set doing hold on: %d %s", got.code, got.errw)
	}
	refused := runCLI(t, root, "move", "fx-1", "doing")
	if refused.code == 0 {
		t.Fatal("the move into the held station succeeded after the hold was turned on")
	}
	if name := refusalNameOf(refused.errw); name != contract.UnresolvedItem {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.UnresolvedItem)
	}
	if !strings.Contains(refused.errw, item) {
		t.Errorf("the refusal names no item; wanted %s in:\n%s", item, refused.errw)
	}

	if got := runCLI(t, root, "set", "doing", bench.HoldField, bench.HoldOff); got.code != 0 {
		t.Fatalf("set doing hold off: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "doing"); got.code != 0 {
		t.Fatalf("the move was refused after the hold was turned off: %d %s", got.code, got.errw)
	}
}

// TestTheStoredHoldKeyReachesNothingAPersonReads is dinah-477 AC-12, which the
// operator minted when he approved the vocabulary. He approved the words and
// not the mechanism, and the stored key is right there in the code, so the
// cheap mistake is letting it out through an answer, a refusal or the help.
// Those are the places this walks.
func TestTheStoredHoldKeyReachesNothingAPersonReads(t *testing.T) {
	root := newBench(t)
	surfaces := map[string]string{}

	written := runCLI(t, root, "--json", "set", "doing", bench.HoldField, bench.HoldOn)
	if written.code != 0 {
		t.Fatalf("set doing hold on: %d %s", written.code, written.errw)
	}
	if !strings.Contains(written.out, `"detail": "`+bench.HoldOn+`"`) {
		t.Errorf("the answer to a write does not carry the word that was typed:\n%s", written.out)
	}
	surfaces["the answer to a write"] = written.out

	read := runCLI(t, root, "get", "doing", bench.HoldField)
	if read.code != 0 {
		t.Fatalf("get doing hold: %d %s", read.code, read.errw)
	}
	if got := strings.TrimSuffix(read.out, "\n"); got != bench.HoldOn {
		t.Errorf("the read answered %q, wanted %q", got, bench.HoldOn)
	}
	surfaces["the answer to a read"] = read.out

	cleared := runCLI(t, root, "--json", "set", "doing", bench.HoldField, bench.HoldOff)
	if cleared.code != 0 {
		t.Fatalf("set doing hold off: %d %s", cleared.code, cleared.errw)
	}
	if !strings.Contains(cleared.out, `"detail": "`+bench.HoldOff+`"`) {
		t.Errorf("the answer to a clearing write does not carry the word that was typed:\n%s", cleared.out)
	}
	surfaces["the answer to a clearing write"] = cleared.out

	// The value typed here is one the stored key does not appear in, because
	// a case that types the stored key and then edits it back out of the
	// answer cannot tell a refusal echoing a person from a refusal
	// volunteering the storage spelling, and an earlier draft of this case
	// did exactly that and could not go red.
	refused := runCLI(t, root, "set", "doing", bench.HoldField, "maybe")
	if refused.code == 0 {
		t.Fatal("a value outside the two was accepted")
	}
	surfaces["the refusal"] = refused.errw

	help := runCLI(t, root, "help", "set")
	if help.code != 0 {
		t.Fatalf("help set: %d %s", help.code, help.errw)
	}
	if !strings.Contains(help.out, bench.HoldField) {
		t.Errorf("the help does not offer the field a person types:\n%s", help.out)
	}
	surfaces["the help"] = help.out

	for where, text := range surfaces {
		if strings.Contains(text, bench.GateItemsKey) {
			t.Errorf("%s spells the stored key:\n%s", where, text)
		}
	}
}

// itemAnchorPath names the file one checklist item of a card is written in, so
// a case can read the bytes a refused write must not have touched.
func itemAnchorPath(t *testing.T, root, card, item string) string {
	t.Helper()
	return filepath.Join(soleBenchDir(t, root), bench.CardsDir, cardID(t, root, card), bench.ChecklistDir, item, bench.ItemAnchor)
}

// TestAnItemColumnSetByItsShortNameHoldsTheCard is dinah-474 AC-3 and AC-5,
// which is the pair that proves the fix reaches the gate rather than only the
// file. The column is named by its slug through the generic write, which is
// the spelling every surface prints and the spelling that used to be stored
// verbatim and hold nothing.
//
// The move is run twice on purpose. A build refusing every move into the held
// station would pass the first half on its own, so the second half settles the
// item and asserts the same move goes through.
func TestAnItemColumnSetByItsShortNameHoldsTheCard(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "Write the release notes"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "set", "doing", bench.HoldField, bench.HoldOn); got.code != 0 {
		t.Fatalf("set doing hold on: %d %s", got.code, got.errw)
	}
	// An acceptance criterion rather than an open question, for the reason
	// TestTheHoldCommandTurnsTheGateOnAndOff gives: an unresolved question
	// refuses a claim wherever the card stands, and this case is about the
	// column's own hold.
	if got := runCLI(t, root, "file", "fx-1", "acceptance_criterion", "Somebody has to settle this."); got.code != 0 {
		t.Fatalf("file: %d %s", got.code, got.errw)
	}
	item := soleItemID(t, root, "fx-1")

	if got := runCLI(t, root, "set", "fx-1/criteria/1", bench.ItemColumnField, "doing"); got.code != 0 {
		t.Fatalf("set the item's column by slug: %d %s", got.code, got.errw)
	}
	stored := runCLI(t, root, "get", "fx-1/criteria/1", bench.ItemColumnField)
	if stored.code != 0 {
		t.Fatalf("read the item's column back: %d %s", stored.code, stored.errw)
	}
	held := columnIdentifier(t, root, "doing")
	if got := strings.TrimSpace(stored.out); got != held {
		t.Fatalf("the item stores the column %q, wanted the identifier %s that the gate compares against", got, held)
	}

	refused := runCLI(t, root, "move", "fx-1", "doing")
	if refused.code == 0 {
		t.Fatal("the move into the held station succeeded carrying the item that names it")
	}
	if name := refusalNameOf(refused.errw); name != contract.UnresolvedItem {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.UnresolvedItem)
	}
	if !strings.Contains(refused.errw, item) {
		t.Errorf("the refusal names no item; wanted %s in:\n%s", item, refused.errw)
	}

	if got := runCLI(t, root, "verify", "fx-1/criteria/1", "the endpoint answers 404, run against the fixture"); got.code != 0 {
		t.Fatalf("verify: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "doing"); got.code != 0 {
		t.Fatalf("the move was refused with the item settled: %d %s", got.code, got.errw)
	}
}

// TestAnItemColumnNamingNoColumnIsRefused is dinah-474 AC-4. The write is
// refused under the name the file path already raises for the same condition,
// and the item's anchor is compared byte for byte, because a refusal that has
// already written is the defect this card exists to close rather than a
// cosmetic one.
func TestAnItemColumnNamingNoColumnIsRefused(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "Write the release notes"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "file", "fx-1", "decision", "Somebody has to settle this."); got.code != 0 {
		t.Fatalf("file: %d %s", got.code, got.errw)
	}
	anchor := itemAnchorPath(t, root, "fx-1", soleItemID(t, root, "fx-1"))
	before, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the item's anchor: %v", err)
	}

	refused := runCLI(t, root, "set", "fx-1/decisions/1", bench.ItemColumnField, "no-such-column")
	if refused.code == 0 {
		t.Fatal("a column nothing resolves was accepted")
	}
	if name := refusalNameOf(refused.errw); name != contract.UnknownColumn {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.UnknownColumn)
	}
	if !strings.Contains(refused.errw, "no-such-column") {
		t.Errorf("the refusal does not name what was typed:\n%s", refused.errw)
	}

	after, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the item's anchor again: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("the refused write changed the anchor:\nbefore\n%s\nafter\n%s", before, after)
	}
}
