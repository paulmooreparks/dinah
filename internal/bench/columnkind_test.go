package bench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// TestTakesWorkUpAnswersForEveryKind is dinah-273 AC-2. The predicate is the
// one place the rule is written, so this table is the one place it is checked:
// no owner takes work up at an intake column, at a done column, at a buffer, or
// wherever the workbench waits on somebody outside, and an owner does at every
// other column including one carrying a kind this build does not implement.
func TestTakesWorkUpAnswersForEveryKind(t *testing.T) {
	cases := []struct {
		name   string
		column Column
		want   bool
	}{
		{name: "an intake column", column: Column{Kind: contract.KindIntake}},
		{name: "a done column", column: Column{Kind: contract.KindDone}},
		{name: "a buffer", column: Column{Kind: contract.KindBuffer}},
		{name: "a work column waiting on somebody outside", column: Column{Kind: contract.KindWork, AwaitingOutside: true}},
		{name: "a plain work column", column: Column{Kind: contract.KindWork}, want: true},
		{name: "a kind this build does not implement", column: Column{Kind: "other.thing"}, want: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.column.TakesWorkUp(); got != c.want {
				t.Errorf("TakesWorkUp: wanted %v, got %v", c.want, got)
			}
		})
	}
}

// TestTerminalAndPullCanTakeFromAnswerForEveryKind is dinah-273 AC-28. The two
// predicates answer different questions and a table over the same columns is
// what shows they do: only a done column ends a card's journey, and a pull may
// take a card out of every column but a done one and one waiting on somebody
// outside.
func TestTerminalAndPullCanTakeFromAnswerForEveryKind(t *testing.T) {
	cases := []struct {
		name        string
		column      Column
		terminal    bool
		canTakeFrom bool
	}{
		{name: "an intake column", column: Column{Kind: contract.KindIntake}, canTakeFrom: true},
		{name: "a plain work column", column: Column{Kind: contract.KindWork}, canTakeFrom: true},
		{name: "a buffer", column: Column{Kind: contract.KindBuffer}, canTakeFrom: true},
		{name: "a kind this build does not implement", column: Column{Kind: "other.thing"}, canTakeFrom: true},
		{name: "a done column", column: Column{Kind: contract.KindDone}, terminal: true},
		{name: "a work column waiting on somebody outside", column: Column{Kind: contract.KindWork, AwaitingOutside: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.column.Terminal(); got != c.terminal {
				t.Errorf("Terminal: wanted %v, got %v", c.terminal, got)
			}
			if got := c.column.PullCanTakeFrom(); got != c.canTakeFrom {
				t.Errorf("PullCanTakeFrom: wanted %v, got %v", c.canTakeFrom, got)
			}
		})
	}
}

// TestStatesAndHoldsStateAgree is dinah-273 AC-3. A column where work is
// taken up holds all three states and one where it is not holds ready and
// blocked, because a block says something about the card rather than about a
// worker. HoldsState is asserted against States for all three in every
// case, so the two cannot answer differently.
func TestStatesAndHoldsStateAgree(t *testing.T) {
	station := Column{Kind: contract.KindWork}
	queues := []struct {
		name   string
		column Column
	}{
		{name: "an intake column", column: Column{Kind: contract.KindIntake}},
		{name: "a done column", column: Column{Kind: contract.KindDone}},
		{name: "a buffer", column: Column{Kind: contract.KindBuffer}},
		{name: "a column waiting on somebody outside", column: Column{Kind: contract.KindWork, AwaitingOutside: true}},
	}
	all := []string{contract.StateReady, contract.StateActive, contract.StateBlocked}

	t.Run("a station", func(t *testing.T) {
		if got := strings.Join(station.States(), ","); got != strings.Join(all, ",") {
			t.Errorf("States: wanted %v, got %v", all, station.States())
		}
		assertHoldsAgreesWithStates(t, station, all)
	})
	for _, c := range queues {
		t.Run(c.name, func(t *testing.T) {
			wanted := []string{contract.StateReady, contract.StateBlocked}
			if got := strings.Join(c.column.States(), ","); got != strings.Join(wanted, ",") {
				t.Errorf("States: wanted %v, got %v", wanted, c.column.States())
			}
			assertHoldsAgreesWithStates(t, c.column, all)
		})
	}
}

// assertHoldsAgreesWithStates checks HoldsState against States for
// every state named, which is what stops the two drifting apart.
func assertHoldsAgreesWithStates(t *testing.T, column Column, states []string) {
	t.Helper()
	held := map[string]bool{}
	for _, state := range column.States() {
		held[state] = true
	}
	for _, state := range states {
		if got := column.HoldsState(state); got != held[state] {
			t.Errorf("HoldsState(%q) answered %v and States says %v", state, got, held[state])
		}
	}
}

// TestABareFourthKindIsStillMalformed is dinah-273 AC-18 and drives
// CORE-STATE-11, which admits a kind this profile declares and a kind carrying
// a layer's prefix and nothing else. `queue` is neither, so a workbench
// carrying it is refused when it is opened.
func TestABareFourthKindIsStillMalformed(t *testing.T) {
	root := kindFixture(t, []map[string]any{
		{"id": "a00000000001", "title": "Intake", "slug": "intake", "kind": "intake"},
		{"id": "a00000000002", "title": "Waiting", "slug": "waiting", "kind": "queue"},
		{"id": "a00000000003", "title": "Done", "slug": "done", "kind": "done"},
	})
	_, err := Open(root)
	if err == nil {
		t.Fatal("a column carrying kind: queue opened, and CORE-STATE-11 admits no such kind")
	}
	refusal := &contract.Refusal{}
	if !asRefusal(err, refusal) || refusal.Name != contract.Malformed {
		t.Fatalf("wanted %s, got %v", contract.Malformed, err)
	}
	// The detail names the offending column, so a refusal raised over some
	// other defect of the fixture cannot pass this test in its place.
	if refusal.Detail != "column a00000000002" {
		t.Errorf("the refusal names %q, and the column carrying the bare kind is a00000000002", refusal.Detail)
	}
}

// TestAKindThisBuildDoesNotImplementOpensAndIsReported is dinah-273 AC-17's
// bench half and drives CORE-STATE-12: a column carrying a layer's kind this
// build does not implement opens, is read as a work column, and dinah check
// names it.
func TestAKindThisBuildDoesNotImplementOpensAndIsReported(t *testing.T) {
	root := kindFixture(t, []map[string]any{
		{"id": "a00000000001", "title": "Intake", "slug": "intake", "kind": "intake"},
		{"id": "a00000000002", "title": "Odd", "slug": "odd", "kind": "other.thing"},
		{"id": "a00000000003", "title": "Done", "slug": "done", "kind": "done"},
	})
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("a workbench carrying a kind this build does not implement should open: %v", err)
	}
	odd := opened.ColumnByRef("odd")
	if odd == nil {
		t.Fatal("the workbench carries no column odd")
	}
	if !odd.TakesWorkUp() {
		t.Error("CORE-STATE-12: a kind this tool does not implement reads as a work column, so work is taken up there")
	}
	assertFindingNames(t, opened, FindingUnknownKind, "odd")
}

// TestAPositionAKindDoesNotAllowIsReported is dinah-273 AC-16. Three flows put
// one kind where its position rule forbids it and each is reported by name,
// and a flow ending in two done columns is reported not at all, which is the
// terminal region the rule is written against.
func TestAPositionAKindDoesNotAllowIsReported(t *testing.T) {
	cases := []struct {
		name    string
		columns []map[string]any
		offends string
	}{
		{
			name: "a buffer standing first",
			columns: []map[string]any{
				{"id": "a00000000001", "title": "Waiting", "slug": "waiting", "kind": "dinah.buffer"},
				{"id": "a00000000002", "title": "Doing", "slug": "doing", "kind": "work"},
				{"id": "a00000000003", "title": "Done", "slug": "done", "kind": "done"},
			},
			offends: "waiting",
		},
		{
			name: "a work column standing after a done column",
			columns: []map[string]any{
				{"id": "a00000000001", "title": "Intake", "slug": "intake", "kind": "intake"},
				{"id": "a00000000002", "title": "Done", "slug": "done", "kind": "done"},
				{"id": "a00000000003", "title": "Doing", "slug": "doing", "kind": "work"},
			},
			offends: "done",
		},
		{
			name: "an intake column standing second",
			columns: []map[string]any{
				{"id": "a00000000001", "title": "Doing", "slug": "doing", "kind": "work"},
				{"id": "a00000000002", "title": "Intake", "slug": "intake", "kind": "intake"},
				{"id": "a00000000003", "title": "Done", "slug": "done", "kind": "done"},
			},
			offends: "intake",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opened, err := Open(kindFixture(t, c.columns))
			if err != nil {
				t.Fatalf("a workbench whose kinds sit outside their positions should open: %v", err)
			}
			assertFindingNames(t, opened, FindingKindOutOfPosition, c.offends)
		})
	}

	t.Run("two done columns at the end of the flow", func(t *testing.T) {
		opened, err := Open(kindFixture(t, []map[string]any{
			{"id": "a00000000001", "title": "Intake", "slug": "intake", "kind": "intake"},
			{"id": "a00000000002", "title": "Doing", "slug": "doing", "kind": "work"},
			{"id": "a00000000003", "title": "Finished", "slug": "finished", "kind": "done"},
			{"id": "a00000000004", "title": "Rejected", "slug": "rejected", "kind": "done"},
		}))
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		findings, err := opened.Check()
		if err != nil {
			t.Fatalf("check: %v", err)
		}
		for _, finding := range findings {
			if finding.Key == FindingKindOutOfPosition {
				t.Errorf("the terminal region holds two done columns and one was reported out of position: %+v", finding)
			}
		}
	})
}

// assertFindingNames fails unless dinah check reports one finding under the
// given key naming the given column.
func assertFindingNames(t *testing.T, b *Bench, key, detail string) {
	t.Helper()
	findings, err := b.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	for _, finding := range findings {
		if finding.Key == key && finding.Detail == detail {
			return
		}
	}
	t.Errorf("check reported %+v, and none of them is %s naming %s", findings, key, detail)
}

// kindFixture writes a workbench carrying the columns given and returns its
// root, which is how each case above gets a flow of its own without a helper
// per shape.
func kindFixture(t *testing.T, columns []map[string]any) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workbench")
	definition := map[string]any{
		"profile":      ProfileVersion,
		"title":        "Kinds",
		"instructions": "Standing text.\n",
		"columns":      columns,
	}
	encoded, err := json.Marshal(definition)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	read, err := ReadDefinition(encoded)
	if err != nil {
		// A definition the interchange reader refuses never reaches Open, and
		// the malformed-kind case is exactly such a definition, so the fixture
		// falls back to writing the anchors by hand.
		writeKindAnchors(t, root, columns, definition)
		return root
	}
	if err := Instantiate(root, "kd", "alka", read); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	return root
}

// writeKindAnchors lays a workbench down file by file, for a definition the
// interchange reader will not accept. Open is the surface under test in those
// cases, so the fixture has to reach it without passing through the reader.
func writeKindAnchors(t *testing.T, root string, columns []map[string]any, definition map[string]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ColumnsDir), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	var listed []string
	for _, column := range columns {
		id, _ := column["id"].(string)
		dir := filepath.Join(root, ColumnsDir, id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir column: %v", err)
		}
		anchor := "---\ntitle: " + column["title"].(string) + "\nslug: " + column["slug"].(string) +
			"\nkind: " + column["kind"].(string) + "\n---\n"
		if err := WriteText(filepath.Join(dir, ColumnAnchor), anchor); err != nil {
			t.Fatalf("write column: %v", err)
		}
		listed = append(listed, "  - "+id)
	}
	if err := os.MkdirAll(filepath.Join(root, CardsDir), 0o755); err != nil {
		t.Fatalf("mkdir cards: %v", err)
	}
	anchor := "---\nformat: 1\nprofile: " + ProfileVersion + "\ntitle: " + definition["title"].(string) +
		"\nslug: kd\noperator: alka\nstates:\n" + strings.Join(listed, "\n") + "\n---\n"
	if err := WriteText(filepath.Join(root, WorkbenchAnchor), anchor); err != nil {
		t.Fatalf("write workbench: %v", err)
	}
}

// asRefusal reports whether an error is a refusal, filling the target in. It
// stands in for errors.As at the one call site that needs it, so the test file
// carries no import for one use.
func asRefusal(err error, target *contract.Refusal) bool {
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		return false
	}
	*target = *refusal
	return true
}
