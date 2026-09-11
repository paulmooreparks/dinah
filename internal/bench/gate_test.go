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

// TestTheGateFlagIsParsedStrictly is dinah-450 AC-1 and dinah-484 AC-1, and
// it is the declaration side of CORE-GATE-1. The value is exactly one of the
// four spellings the field carries, following awaiting_outside's strictness
// rather than operator_owned, whose == "true" test reads yes as false and
// tells nobody it did.
//
// true is the spelling every workbench written before the direction existed
// carries, and it goes on reading as the entry direction, which is what makes
// this card's addition invisible to a workbench that never heard of it.
func TestTheGateFlagIsParsedStrictly(t *testing.T) {
	t.Run("a value outside the four refuses the workbench", func(t *testing.T) {
		for _, value := range []string{"yes", "1", "True", "no", "bogus", "in", "On", "Out", "exit"} {
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

	t.Run("each declared value reads as the direction it names", func(t *testing.T) {
		for _, want := range []struct {
			stored string
			hold   string
			entry  bool
			exit   bool
		}{
			{"true", HoldOn, true, false},
			{"false", "", false, false},
			{"out", HoldOut, false, true},
			{"both", HoldBoth, true, true},
		} {
			root := newFixture(t)
			write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor),
				"---\ntitle: Only\nslug: only\nkind: work\ngate_items: "+want.stored+"\n---\nColumn text.\n")
			opened, err := Open(root)
			if err != nil {
				t.Fatalf("gate_items: %s refused the workbench: %v", want.stored, err)
			}
			column := opened.Columns[0]
			if column.Hold != want.hold {
				t.Errorf("gate_items: %s read as %q, wanted %q", want.stored, column.Hold, want.hold)
			}
			if column.HoldsOnEntry() != want.entry {
				t.Errorf("gate_items: %s holds on entry %v, wanted %v", want.stored, column.HoldsOnEntry(), want.entry)
			}
			if column.HoldsOnExit() != want.exit {
				t.Errorf("gate_items: %s holds on exit %v, wanted %v", want.stored, column.HoldsOnExit(), want.exit)
			}
		}
	})

	t.Run("a workbench with no such key opens unchanged", func(t *testing.T) {
		root := newFixture(t)
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		if opened.Columns[0].Hold != "" {
			t.Errorf("a column that declares nothing read as %q", opened.Columns[0].Hold)
		}
		if opened.Columns[0].HoldsOnEntry() || opened.Columns[0].HoldsOnExit() {
			t.Error("a column that declares nothing holds a card")
		}
		if opened.Columns[0].FM.Value("gate_items") != "" {
			t.Error("opening a workbench invented the key")
		}
	})
}

// TestHoldsOnEntryAndExitReadTheFourStates runs the two predicates over every
// value the field takes, including the empty one, so a value added later
// without a reading here fails rather than answering false quietly.
func TestHoldsOnEntryAndExitReadTheFourStates(t *testing.T) {
	for _, want := range []struct {
		hold  string
		entry bool
		exit  bool
	}{
		{"", false, false},
		{HoldOn, true, false},
		{HoldOut, false, true},
		{HoldBoth, true, true},
	} {
		column := &Column{Hold: want.hold}
		if got := column.HoldsOnEntry(); got != want.entry {
			t.Errorf("HoldsOnEntry on %q is %v, wanted %v", want.hold, got, want.entry)
		}
		if got := column.HoldsOnExit(); got != want.exit {
			t.Errorf("HoldsOnExit on %q is %v, wanted %v", want.hold, got, want.exit)
		}
	}
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
	if opened.Columns[0].Hold != "" || opened.Columns[1].Hold != HoldOn {
		t.Fatalf("wanted the flag on the second column alone, got %q and %q",
			opened.Columns[0].Hold, opened.Columns[1].Hold)
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
	if reopened.Columns[1].Hold != HoldOn {
		t.Errorf("the import read the flag back as %q", reopened.Columns[1].Hold)
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

		got := GatingItems(card, held)
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
		if got := GatingItems(card, held); len(got) != 1 {
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

			got := GatingItems(card, held)
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
		if got := GatingItems(card, held); len(got) != 0 {
			t.Fatalf("wanted nothing held, got %s", ids(got))
		}
	})

	t.Run("asked with no column at all, nothing is held", func(t *testing.T) {
		card := t.TempDir()
		plantChecklistItem(t, card, "b00000000001",
			"kind: decision\nstate: pending\nordinal: 1\n", "A decision naming no column.")
		if got := GatingItems(card, ""); len(got) != 0 {
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
	if got := GatingItems(card, "e00000000002"); len(got) != 1 || got[0].ID != "b00000000001" {
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
		if got := GatingItems(card, held); len(got) != 1 || got[0].ID != "b00000000001" {
			t.Fatalf("wanted the failed criterion held, got %s", ids(got))
		}
	})
}

// directedDefinition is a four-column workbench declaring every state the
// hold takes, in the order off, on, out, both, which is the smallest fixture
// that can show each direction exported under its own spelling and left off
// the column that declares none.
const directedDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Directed",
  "columns": [
    { "id": "e00000000001", "title": "Plain", "kind": "work",
      "instructions": "Plain instructions.\n" },
    { "id": "e00000000002", "title": "Entry", "kind": "work",
      "gate_items": true, "instructions": "Entry instructions.\n" },
    { "id": "e00000000003", "title": "Exit", "kind": "work",
      "gate_items": "out", "instructions": "Exit instructions.\n" },
    { "id": "e00000000004", "title": "Either", "kind": "work",
      "gate_items": "both", "instructions": "Either instructions.\n" }
  ]
}`

// TestEveryHoldDirectionRidesTheInterchange is dinah-484 AC-3. The member
// CORE-JSON-10 blesses now carries a boolean for the direction the profile
// knows and a string for the two Dinah adds, so the round trip is what catches
// a direction the parser reads and exportColumn drops, which is the half of
// the interchange nobody notices until a workbench is carried somewhere.
func TestEveryHoldDirectionRidesTheInterchange(t *testing.T) {
	first := containedPath(filepath.Join(t.TempDir(), "first"))
	definition, err := ReadDefinition([]byte(directedDefinition))
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
	wantHolds := []string{"", HoldOn, HoldOut, HoldBoth}
	for i, want := range wantHolds {
		if got := opened.Columns[i].Hold; got != want {
			t.Fatalf("column %d reads the hold %q, wanted %q", i, got, want)
		}
	}

	exported, err := opened.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	columns := exportedColumns(t, exported)
	if len(columns) != 4 {
		t.Fatalf("wanted four columns in the export, got %d", len(columns))
	}
	if _, carried := columns[0]["gate_items"]; carried {
		t.Error("the export carried the member on a column that declares no hold")
	}
	var flag bool
	if err := json.Unmarshal(columns[1]["gate_items"], &flag); err != nil || !flag {
		t.Errorf("the entry column exported as %s, wanted the boolean true", columns[1]["gate_items"])
	}
	for i, want := range map[int]string{2: HoldOut, 3: HoldBoth} {
		var word string
		if err := json.Unmarshal(columns[i]["gate_items"], &word); err != nil || word != want {
			t.Errorf("column %d exported as %s, wanted the string %q", i, columns[i]["gate_items"], want)
		}
	}

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
	for i, want := range wantHolds {
		if got := reopened.Columns[i].Hold; got != want {
			t.Errorf("the import read column %d back as %q, wanted %q", i, got, want)
		}
	}
	again, err := reopened.Export()
	if err != nil {
		t.Fatalf("export the import: %v", err)
	}
	if string(again) != string(exported) {
		t.Errorf("the round trip changed the export:\nfirst:\n%s\nsecond:\n%s", exported, again)
	}
}

// TestAnUnreadableHoldMemberIsDroppedRatherThanRefused is dinah-484 AC-3's
// last row. The import side is lenient on purpose, matching what
// awaiting_outside and operator_owned already do with a member of the wrong
// shape: one column's one member is not worth refusing a whole workbench over,
// and a build that refused instead would make a workbench written by a newer
// Dinah unreadable by an older one.
func TestAnUnreadableHoldMemberIsDroppedRatherThanRefused(t *testing.T) {
	for _, shape := range []string{`3`, `["out"]`, `{"direction":"out"}`, `"sideways"`, `"in"`} {
		definition, err := ReadDefinition([]byte(`{
  "profile": "dinah-core/0.12",
  "title": "Odd",
  "columns": [
    { "id": "e00000000001", "title": "Only", "kind": "work",
      "gate_items": ` + shape + `, "instructions": "Only instructions.\n" }
  ]
}`))
		if err != nil {
			t.Fatalf("gate_items: %s: definition: %v", shape, err)
		}
		root := containedPath(filepath.Join(t.TempDir(), "wb"))
		if err := Instantiate(root, "wt", "alka", definition); err != nil {
			t.Fatalf("gate_items: %s: instantiate: %v", shape, err)
		}
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("gate_items: %s: the import refused the workbench: %v", shape, err)
		}
		if got := opened.Columns[0].Hold; got != "" {
			t.Errorf("gate_items: %s was written to disk as the hold %q, wanted it dropped", shape, got)
		}
		if got := opened.Columns[0].FM.Value("gate_items"); got != "" {
			t.Errorf("gate_items: %s reached the anchor as %q, wanted the key left off", shape, got)
		}
	}
}

// TestTheOwnerFieldNamesWhatEnforcesIt is dinah-484 AC-7's last row. The doc
// comment on ItemOwnerField used to say the field was recorded and never
// enforced against the actor calling a terminal verb, which stopped being true
// on this card, and a reader who trusts the old sentence would file an item in
// the operator's name believing it protects nothing.
//
// The guard reads the source rather than the behaviour on purpose. What it
// catches is a sentence, and no run of the tool can go red over one.
func TestTheOwnerFieldNamesWhatEnforcesIt(t *testing.T) {
	text, err := os.ReadFile("item.go")
	if err != nil {
		t.Fatalf("read item.go: %v", err)
	}
	if strings.Contains(string(text), "recorded and never") {
		t.Error("item.go still says the owner field is recorded and never enforced against the actor calling a terminal verb")
	}
	for _, named := range []string{"closeItem", "SetField"} {
		if !strings.Contains(string(text), named) {
			t.Errorf("item.go's owner field names no %s, so nothing there says what enforces the owner", named)
		}
	}
}
