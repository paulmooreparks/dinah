package bench

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// gatedDefinition is a two-column workbench whose second station holds a card
// while an item on it names that station, which is the smallest fixture that
// can show the member written on one column and left off the other.
const gatedDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Gated",
  "columns": [
    { "id": "e00000000001", "title": "Doing", "kind": "work",
      "instructions": "Doing instructions.\n" },
    { "id": "e00000000002", "title": "Merge", "kind": "work",
      "gate_items": true, "instructions": "Merge instructions.\n" }
  ]
}`

// TestTheGateFlagIsParsedStrictly is dinah-450 AC-1 and the declaration side
// of CORE-GATE-1. The value is exactly true or false, following
// awaiting_outside rather than operator_owned, whose == "true" test reads yes
// as false and tells nobody it did.
func TestTheGateFlagIsParsedStrictly(t *testing.T) {
	t.Run("a value outside the two refuses the workbench", func(t *testing.T) {
		for _, value := range []string{"yes", "1", "True", "no", "bogus"} {
			root := newFixture(t)
			write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor),
				"---\ntitle: Only\nslug: only\nkind: work\ngate_items: "+value+"\n---\nColumn text.\n")
			_, err := Open(root)
			if err == nil {
				t.Fatalf("gate_items: %s opened the workbench", value)
			}
			var refusal *contract.Refusal
			if !errors.As(err, &refusal) || refusal.Name != contract.Malformed {
				t.Fatalf("gate_items: %s: wanted %s, got %v", value, contract.Malformed, err)
			}
			if !strings.Contains(refusal.Detail, "b00000000001") {
				t.Errorf("the refusal should name the column, got %q", refusal.Detail)
			}
		}
	})

	t.Run("true reads as a column that holds", func(t *testing.T) {
		root := newFixture(t)
		write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor),
			"---\ntitle: Only\nslug: only\nkind: work\ngate_items: true\n---\nColumn text.\n")
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("gate_items: true refused the workbench: %v", err)
		}
		if !opened.Columns[0].GateItems {
			t.Error("gate_items: true read as false")
		}
	})

	t.Run("false reads as a column that never carried the key", func(t *testing.T) {
		root := newFixture(t)
		write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor),
			"---\ntitle: Only\nslug: only\nkind: work\ngate_items: false\n---\nColumn text.\n")
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("gate_items: false refused the workbench: %v", err)
		}
		if opened.Columns[0].GateItems {
			t.Error("gate_items: false read as true")
		}
	})

	t.Run("a workbench with no such key opens unchanged", func(t *testing.T) {
		root := newFixture(t)
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		if opened.Columns[0].GateItems {
			t.Error("a column that declares nothing read as holding")
		}
		if opened.Columns[0].FM.Value("gate_items") != "" {
			t.Error("opening a workbench invented the key")
		}
	})
}

// TestTheGateFlagRidesTheInterchange is dinah-450 AC-2, and it is the member
// CORE-JSON-10 blesses. The round trip is what catches a field added to the
// parser and forgotten in exportColumn or in knownColumnKeys, which is the
// half of the interchange nobody notices until a workbench is carried
// somewhere.
func TestTheGateFlagRidesTheInterchange(t *testing.T) {
	first := containedPath(filepath.Join(t.TempDir(), "first"))
	definition, err := ReadDefinition([]byte(gatedDefinition))
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	if err := Instantiate(first, "wt", "alka", definition); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	opened, err := Open(first)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if opened.Columns[0].GateItems || !opened.Columns[1].GateItems {
		t.Fatalf("wanted the flag on the second column alone, got %v and %v",
			opened.Columns[0].GateItems, opened.Columns[1].GateItems)
	}
	exported, err := opened.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	columns := exportedColumns(t, exported)
	if len(columns) != 2 {
		t.Fatalf("wanted two columns in the export, got %d", len(columns))
	}
	if _, carried := columns[0]["gate_items"]; carried {
		t.Error("the export carried the member on a column that does not declare it")
	}
	var flag bool
	raw, carried := columns[1]["gate_items"]
	if !carried {
		t.Fatal("the export left the member off the column that declares it")
	}
	if err := json.Unmarshal(raw, &flag); err != nil || !flag {
		t.Errorf("the member exported as %s, wanted true", raw)
	}

	// init --from reads the export back, which is the import half, and an
	// export of the result matching the first byte for byte is what proves
	// nothing was dropped or invented in between.
	second := containedPath(filepath.Join(t.TempDir(), "second"))
	reread, err := ReadDefinition(exported)
	if err != nil {
		t.Fatalf("read the export back: %v", err)
	}
	if err := Instantiate(second, "wt", "alka", reread); err != nil {
		t.Fatalf("instantiate the import: %v", err)
	}
	reopened, err := Open(second)
	if err != nil {
		t.Fatalf("open the import: %v", err)
	}
	if !reopened.Columns[1].GateItems {
		t.Error("the import dropped the flag")
	}
	again, err := reopened.Export()
	if err != nil {
		t.Fatalf("export the import: %v", err)
	}
	if string(again) != string(exported) {
		t.Errorf("the round trip changed the export:\nfirst:\n%s\nsecond:\n%s", exported, again)
	}
}

// TestItemIsResolvedAnswersTheSameForEveryKind is dinah-450 AC-3. The table
// runs all three kinds across all four states, because the whole point of the
// extraction is that the state alone decides, and a reading that kept a kind
// in it would pass a table that exercised one kind.
//
// The second half is the guard on what did not change: ItemBlocksClaim goes on
// exempting an acceptance criterion in every state, which is CORE-CLAIM-9's
// own ruling and not this card's to move, and it goes on reading failed as
// settled, which the column hold no longer does.
func TestItemIsResolvedAnswersTheSameForEveryKind(t *testing.T) {
	states := []struct {
		state    string
		resolved bool
		lifts    bool
	}{
		{ItemPending, false, false},
		{ItemResolved, true, true},
		{ItemVerified, true, true},
		{ItemFailed, true, false},
		{"", false, false},
		{"halfway", false, false},
	}
	for _, kind := range ItemKinds {
		for _, want := range states {
			item := &Item{ID: "b00000000001", Kind: kind, State: want.state}
			if got := ItemIsResolved(item); got != want.resolved {
				t.Errorf("ItemIsResolved(%s in %q) is %v, wanted %v", kind, want.state, got, want.resolved)
			}
			if got := ItemLiftsColumnHold(item); got != want.lifts {
				t.Errorf("ItemLiftsColumnHold(%s in %q) is %v, wanted %v", kind, want.state, got, want.lifts)
			}
			blocks := kind != "acceptance_criterion" && !want.resolved
			if got := ItemBlocksClaim(item); got != blocks {
				t.Errorf("ItemBlocksClaim(%s in %q) is %v, wanted %v", kind, want.state, got, blocks)
			}
		}
	}
}

// TestGatingItemsExcludesNoKind is dinah-450 AC-4, and it is AC-8's first half
// read at the function the refusal calls. A build that inherited
// ItemBlocksClaim's exemption would answer two here where three are wanted,
// and the subtest below that files a criterion alone is the one that says so
// in as many words.
func TestGatingItemsExcludesNoKind(t *testing.T) {
	held := "e00000000002"

	t.Run("every kind naming the column is held", func(t *testing.T) {
		card := t.TempDir()
		plantChecklistItem(t, card, "b00000000001",
			"kind: open_question\nstate: pending\ncolumn: "+held+"\nordinal: 1\n", "A question.")
		plantChecklistItem(t, card, "b00000000002",
			"kind: decision\nstate: pending\ncolumn: "+held+"\nordinal: 2\n", "A decision.")
		plantChecklistItem(t, card, "b00000000003",
			"kind: acceptance_criterion\nstate: pending\ncolumn: "+held+"\nordinal: 3\n", "A criterion.")

		got, gotErr7 := GatingItems(card, held)
		if gotErr7 != nil {
			t.Fatalf("GatingItems: %v", gotErr7)
		}
		if len(got) != 3 {
			t.Fatalf("wanted all three kinds held, got %d: %s", len(got), ids(got))
		}
		if names := ids(got); names != "b00000000001 b00000000002 b00000000003" {
			t.Errorf("wanted the three in identifier order, got %s", names)
		}
	})

	t.Run("an acceptance criterion alone is held", func(t *testing.T) {
		// The kind claim exempts unconditionally is the one this hold was
		// reopened for, so it is asserted on its own rather than only in
		// company, where two other holding items would hide its absence.
		card := t.TempDir()
		plantChecklistItem(t, card, "b00000000001",
			"kind: acceptance_criterion\nstate: pending\ncolumn: "+held+"\nordinal: 1\n", "A criterion.")
		got6, gotErr6 := GatingItems(card, held)
		if gotErr6 != nil {
			t.Fatalf("GatingItems: %v", gotErr6)
		}
		if got := got6; len(got) != 1 {
			t.Fatalf("wanted the criterion held, got %d: %s", len(got), ids(got))
		}
	})

	t.Run("an item resolved or verified is not held", func(t *testing.T) {
		for _, state := range []string{ItemResolved, ItemVerified} {
			card := t.TempDir()
			plantChecklistItem(t, card, "b00000000001",
				"kind: open_question\nstate: "+state+"\ncolumn: "+held+"\nnote: settled\nordinal: 1\n", "A question.")
			plantChecklistItem(t, card, "b00000000002",
				"kind: decision\nstate: pending\ncolumn: "+held+"\nordinal: 2\n", "A decision.")
			plantChecklistItem(t, card, "b00000000003",
				"kind: acceptance_criterion\nstate: pending\ncolumn: "+held+"\nordinal: 3\n", "A criterion.")

			got, gotErr5 := GatingItems(card, held)
			if gotErr5 != nil {
				t.Fatalf("GatingItems: %v", gotErr5)
			}
			if names := ids(got); names != "b00000000002 b00000000003" {
				t.Errorf("with the question %s, wanted the other two held, got %s", state, names)
			}
		}
	})

	t.Run("an item naming another column holds nothing here", func(t *testing.T) {
		card := t.TempDir()
		plantChecklistItem(t, card, "b00000000001",
			"kind: decision\nstate: pending\ncolumn: e00000000001\nordinal: 1\n", "A decision.")
		plantChecklistItem(t, card, "b00000000002",
			"kind: decision\nstate: pending\nordinal: 2\n", "A decision naming no column.")
		got4, gotErr4 := GatingItems(card, held)
		if gotErr4 != nil {
			t.Fatalf("GatingItems: %v", gotErr4)
		}
		if got := got4; len(got) != 0 {
			t.Fatalf("wanted nothing held, got %s", ids(got))
		}
	})

	t.Run("asked with no column at all, nothing is held", func(t *testing.T) {
		card := t.TempDir()
		plantChecklistItem(t, card, "b00000000001",
			"kind: decision\nstate: pending\nordinal: 1\n", "A decision naming no column.")
		got3, gotErr3 := GatingItems(card, "")
		if gotErr3 != nil {
			t.Fatalf("GatingItems: %v", gotErr3)
		}
		if got := got3; len(got) != 0 {
			t.Fatalf("an empty column held %s, and an item naming no column names no column", ids(got))
		}
	})
}

// ids names the items a read came back with, so a failure prints what was held
// rather than a count and a pointer.
func ids(items []*Item) string {
	var names []string
	for _, item := range items {
		names = append(names, item.ID)
	}
	return strings.Join(names, " ")
}

// TestGatingItemsSkipsAnItemWhoseAnchorWillNotOpen holds the gate read to the
// tolerance BlockingItems beside it already has, which is the shared reader
// they now both call.
func TestGatingItemsSkipsAnItemWhoseAnchorWillNotOpen(t *testing.T) {
	card := t.TempDir()
	plantChecklistItem(t, card, "b00000000001",
		"kind: decision\nstate: pending\ncolumn: e00000000002\nordinal: 1\n", "A decision.")
	if err := os.MkdirAll(filepath.Join(card, ChecklistDir, "b00000000002"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	got2, gotErr2 := GatingItems(card, "e00000000002")
	if gotErr2 != nil {
		t.Fatalf("GatingItems: %v", gotErr2)
	}
	if got := got2; len(got) != 1 || got[0].ID != "b00000000001" {
		t.Fatalf("wanted the one readable item, got %s", ids(got))
	}
}

// TestAFailedItemHoldsAColumnThatTheClaimRefusalLetsThrough is the operator's
// ruling of 2026-09-10, recorded as dinah-450 OQ-5, at the predicate the two
// holds part company on.
//
// It names the failed state on purpose. The gate held a pending item before
// the ruling and holds one after it, so a guard written against the hold in
// general stays green on the build this test exists to refuse, which is the
// build where a criterion somebody checked and found wanting opens the very
// column meant to stop it.
//
// The second half is what the ruling deliberately left alone. The claim
// refusal goes on reading failed as settled: it exempts acceptance criteria
// outright, so the ruling does not reach it, and narrowing the state set
// underneath it would move behaviour nobody ruled on.
func TestAFailedItemHoldsAColumnThatTheClaimRefusalLetsThrough(t *testing.T) {
	held := "e00000000002"
	for _, kind := range ItemKinds {
		item := &Item{ID: "b00000000001", Kind: kind, State: ItemFailed, Column: held}
		if ItemLiftsColumnHold(item) {
			t.Errorf("a failed %s lifted the column hold", kind)
		}
		if !ItemIsResolved(item) {
			t.Errorf("a failed %s stopped reading as resolved, which the claim refusal turns on", kind)
		}
		if ItemBlocksClaim(item) {
			t.Errorf("a failed %s began blocking a claim, which no ruling asked for", kind)
		}
	}

	t.Run("read off disk at the gate", func(t *testing.T) {
		card := t.TempDir()
		plantChecklistItem(t, card, "b00000000001",
			"kind: acceptance_criterion\nstate: "+ItemFailed+"\ncolumn: "+held+"\nnote: the endpoint still answers 200\nordinal: 1\n", "A criterion.")
		got1, gotErr1 := GatingItems(card, held)
		if gotErr1 != nil {
			t.Fatalf("GatingItems: %v", gotErr1)
		}
		if got := got1; len(got) != 1 || got[0].ID != "b00000000001" {
			t.Fatalf("wanted the failed criterion held, got %s", ids(got))
		}
	})
}
