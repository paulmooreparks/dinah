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
// no owner takes work up at an intake state, at a done state, at a buffer, or
// wherever the workbench waits on somebody outside, and an owner does at every
// other state including one carrying a kind this build does not implement.
func TestTakesWorkUpAnswersForEveryKind(t *testing.T) {
	cases := []struct {
		name  string
		state State
		want  bool
	}{
		{name: "an intake state", state: State{Kind: contract.KindIntake}},
		{name: "a done state", state: State{Kind: contract.KindDone}},
		{name: "a buffer", state: State{Kind: contract.KindBuffer}},
		{name: "a work state waiting on somebody outside", state: State{Kind: contract.KindWork, AwaitingOutside: true}},
		{name: "a plain work state", state: State{Kind: contract.KindWork}, want: true},
		{name: "a kind this build does not implement", state: State{Kind: "other.thing"}, want: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.state.TakesWorkUp(); got != c.want {
				t.Errorf("TakesWorkUp: wanted %v, got %v", c.want, got)
			}
		})
	}
}

// TestTerminalAndPullCanTakeFromAnswerForEveryKind is dinah-273 AC-28. The two
// predicates answer different questions and a table over the same states is
// what shows they do: only a done state ends a card's journey, and a pull may
// take a card out of every state but a done one and one waiting on somebody
// outside.
func TestTerminalAndPullCanTakeFromAnswerForEveryKind(t *testing.T) {
	cases := []struct {
		name        string
		state       State
		terminal    bool
		canTakeFrom bool
	}{
		{name: "an intake state", state: State{Kind: contract.KindIntake}, canTakeFrom: true},
		{name: "a plain work state", state: State{Kind: contract.KindWork}, canTakeFrom: true},
		{name: "a buffer", state: State{Kind: contract.KindBuffer}, canTakeFrom: true},
		{name: "a kind this build does not implement", state: State{Kind: "other.thing"}, canTakeFrom: true},
		{name: "a done state", state: State{Kind: contract.KindDone}, terminal: true},
		{name: "a work state waiting on somebody outside", state: State{Kind: contract.KindWork, AwaitingOutside: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.state.Terminal(); got != c.terminal {
				t.Errorf("Terminal: wanted %v, got %v", c.terminal, got)
			}
			if got := c.state.PullCanTakeFrom(); got != c.canTakeFrom {
				t.Errorf("PullCanTakeFrom: wanted %v, got %v", c.canTakeFrom, got)
			}
		})
	}
}

// TestSubstatesAndHoldsSubstateAgree is dinah-273 AC-3. A state where work is
// taken up holds all three substates and one where it is not holds ready and
// blocked, because a block says something about the card rather than about a
// worker. HoldsSubstate is asserted against Substates for all three in every
// case, so the two cannot answer differently.
func TestSubstatesAndHoldsSubstateAgree(t *testing.T) {
	station := State{Kind: contract.KindWork}
	queues := []struct {
		name  string
		state State
	}{
		{name: "an intake state", state: State{Kind: contract.KindIntake}},
		{name: "a done state", state: State{Kind: contract.KindDone}},
		{name: "a buffer", state: State{Kind: contract.KindBuffer}},
		{name: "a state waiting on somebody outside", state: State{Kind: contract.KindWork, AwaitingOutside: true}},
	}
	all := []string{contract.SubstateReady, contract.SubstateActive, contract.SubstateBlocked}

	t.Run("a station", func(t *testing.T) {
		if got := strings.Join(station.Substates(), ","); got != strings.Join(all, ",") {
			t.Errorf("Substates: wanted %v, got %v", all, station.Substates())
		}
		assertHoldsAgreesWithSubstates(t, station, all)
	})
	for _, c := range queues {
		t.Run(c.name, func(t *testing.T) {
			wanted := []string{contract.SubstateReady, contract.SubstateBlocked}
			if got := strings.Join(c.state.Substates(), ","); got != strings.Join(wanted, ",") {
				t.Errorf("Substates: wanted %v, got %v", wanted, c.state.Substates())
			}
			assertHoldsAgreesWithSubstates(t, c.state, all)
		})
	}
}

// assertHoldsAgreesWithSubstates checks HoldsSubstate against Substates for
// every substate named, which is what stops the two drifting apart.
func assertHoldsAgreesWithSubstates(t *testing.T, state State, substates []string) {
	t.Helper()
	held := map[string]bool{}
	for _, substate := range state.Substates() {
		held[substate] = true
	}
	for _, substate := range substates {
		if got := state.HoldsSubstate(substate); got != held[substate] {
			t.Errorf("HoldsSubstate(%q) answered %v and Substates says %v", substate, got, held[substate])
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
		t.Fatal("a state carrying kind: queue opened, and CORE-STATE-11 admits no such kind")
	}
	refusal := &contract.Refusal{}
	if !asRefusal(err, refusal) || refusal.Name != contract.Malformed {
		t.Fatalf("wanted %s, got %v", contract.Malformed, err)
	}
	// The detail names the offending state, so a refusal raised over some
	// other defect of the fixture cannot pass this test in its place.
	if refusal.Detail != "state a00000000002" {
		t.Errorf("the refusal names %q, and the state carrying the bare kind is a00000000002", refusal.Detail)
	}
}

// TestAKindThisBuildDoesNotImplementOpensAndIsReported is dinah-273 AC-17's
// bench half and drives CORE-STATE-12: a state carrying a layer's kind this
// build does not implement opens, is read as a work state, and dinah check
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
	odd := opened.StateByRef("odd")
	if odd == nil {
		t.Fatal("the workbench carries no state odd")
	}
	if !odd.TakesWorkUp() {
		t.Error("CORE-STATE-12: a kind this tool does not implement reads as a work state, so work is taken up there")
	}
	assertFindingNames(t, opened, FindingUnknownKind, "odd")
}

// TestAPositionAKindDoesNotAllowIsReported is dinah-273 AC-16. Three flows put
// one kind where its position rule forbids it and each is reported by name,
// and a flow ending in two done states is reported not at all, which is the
// terminal region the rule is written against.
func TestAPositionAKindDoesNotAllowIsReported(t *testing.T) {
	cases := []struct {
		name    string
		states  []map[string]any
		offends string
	}{
		{
			name: "a buffer standing first",
			states: []map[string]any{
				{"id": "a00000000001", "title": "Waiting", "slug": "waiting", "kind": "dinah.buffer"},
				{"id": "a00000000002", "title": "Doing", "slug": "doing", "kind": "work"},
				{"id": "a00000000003", "title": "Done", "slug": "done", "kind": "done"},
			},
			offends: "waiting",
		},
		{
			name: "a work state standing after a done state",
			states: []map[string]any{
				{"id": "a00000000001", "title": "Intake", "slug": "intake", "kind": "intake"},
				{"id": "a00000000002", "title": "Done", "slug": "done", "kind": "done"},
				{"id": "a00000000003", "title": "Doing", "slug": "doing", "kind": "work"},
			},
			offends: "done",
		},
		{
			name: "an intake state standing second",
			states: []map[string]any{
				{"id": "a00000000001", "title": "Doing", "slug": "doing", "kind": "work"},
				{"id": "a00000000002", "title": "Intake", "slug": "intake", "kind": "intake"},
				{"id": "a00000000003", "title": "Done", "slug": "done", "kind": "done"},
			},
			offends: "intake",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opened, err := Open(kindFixture(t, c.states))
			if err != nil {
				t.Fatalf("a workbench whose kinds sit outside their positions should open: %v", err)
			}
			assertFindingNames(t, opened, FindingKindOutOfPosition, c.offends)
		})
	}

	t.Run("two done states at the end of the flow", func(t *testing.T) {
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
				t.Errorf("the terminal region holds two done states and one was reported out of position: %+v", finding)
			}
		}
	})
}

// assertFindingNames fails unless dinah check reports one finding under the
// given key naming the given state.
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

// kindFixture writes a workbench carrying the states given and returns its
// root, which is how each case above gets a flow of its own without a helper
// per shape.
func kindFixture(t *testing.T, states []map[string]any) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workbench")
	definition := map[string]any{
		"profile":      ProfileVersion,
		"title":        "Kinds",
		"instructions": "Standing text.\n",
		"states":       states,
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
		writeKindAnchors(t, root, states, definition)
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
func writeKindAnchors(t *testing.T, root string, states []map[string]any, definition map[string]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, StatesDir), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	var listed []string
	for _, state := range states {
		id, _ := state["id"].(string)
		dir := filepath.Join(root, StatesDir, id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir state: %v", err)
		}
		anchor := "---\ntitle: " + state["title"].(string) + "\nslug: " + state["slug"].(string) +
			"\nkind: " + state["kind"].(string) + "\n---\n"
		if err := WriteText(filepath.Join(dir, StateAnchor), anchor); err != nil {
			t.Fatalf("write state: %v", err)
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
