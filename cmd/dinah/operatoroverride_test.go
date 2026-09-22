package main

import (
	"strings"
	"testing"

	"dinah/internal/contract"
)

// dinah-571's own follow-up: the exit hold was not the only refusal
// req.Override passes. canLand gates five refusals behind !req.Override
// (verified below by grep rather than by trust), and canRoute refuses
// req.Override to anybody but the operator before any of them is reached, so
// every one of them is, in practice, a row only the operator can pass. Each
// of the four tests below pins one of the remaining four: the message the
// operator reads names --override, and the message anybody else reads does
// not, on the same terms exithold_test.go already pins for the fifth,
// UnresolvedItemExit.

// atCapacityDefinition is a three-column flow whose working station admits
// one card at a time.
const atCapacityDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "At capacity",
  "columns": [
    { "id": "g00000000001", "title": "Intake", "kind": "intake" },
    { "id": "g00000000002", "title": "Doing", "kind": "work", "capacity": 1 },
    { "id": "g00000000003", "title": "Done", "kind": "done" }
  ]
}`

// TestTheOperatorRefusalAtCapacityNamesOverride is dinah-571's own case for
// the first of the four remaining overridable refusals.
func TestTheOperatorRefusalAtCapacityNamesOverride(t *testing.T) {
	build := func(t *testing.T) string {
		t.Helper()
		root := newBenchFromDefinition(t, atCapacityDefinition)
		for _, title := range []string{"First", "Second"} {
			if got := runCLI(t, root, "add", title); got.code != 0 {
				t.Fatalf("add %s: %d %s", title, got.code, got.errw)
			}
		}
		if got := runCLI(t, root, "move", "fx-1", "doing"); got.code != 0 {
			t.Fatalf("fill the column: %d %s", got.code, got.errw)
		}
		return root
	}

	operatorRoot := build(t)
	operator := runCLI(t, operatorRoot, "move", "fx-2", "doing")
	if operator.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("the operator's move exited %d, wanted %d", operator.code, contract.ExitCode(contract.OutcomeRefused))
	}
	if name := refusalNameOf(operator.errw); name != contract.AtCapacity {
		t.Fatalf("the refusal name is %s, wanted %s", name, contract.AtCapacity)
	}
	if !strings.Contains(operator.errw, "--override") {
		t.Errorf("the operator's own refusal does not name --override:\n%s", operator.errw)
	}

	otherRoot := build(t)
	t.Setenv("DINAH_ACTOR", "sam")
	other := runCLI(t, otherRoot, "move", "fx-2", "doing")
	if other.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("the non-operator's move exited %d, wanted %d", other.code, contract.ExitCode(contract.OutcomeRefused))
	}
	if name := refusalNameOf(other.errw); name != contract.AtCapacity {
		t.Fatalf("the refusal name is %s, wanted %s", name, contract.AtCapacity)
	}
	if strings.Contains(other.errw, "--override") {
		t.Errorf("a non-operator's refusal names --override, which he cannot pass:\n%s", other.errw)
	}
}

// entryOnlyOverrideDefinition declares an entry hold with nothing filed
// against it yet, so the fixture files one item per case, on a fresh card.
const entryOnlyOverrideDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Entry hold override",
  "columns": [
    { "id": "g10000000001", "title": "Intake", "kind": "intake" },
    { "id": "g10000000002", "title": "Doing", "kind": "work", "gate_items": true },
    { "id": "g10000000003", "title": "Done", "kind": "done" }
  ]
}`

// TestTheOperatorRefusalAtTheEntryHoldNamesOverride is dinah-571's own case
// for the second of the four remaining overridable refusals.
func TestTheOperatorRefusalAtTheEntryHoldNamesOverride(t *testing.T) {
	build := func(t *testing.T) string {
		t.Helper()
		root := newBenchFromDefinition(t, entryOnlyOverrideDefinition)
		if got := runCLI(t, root, "add", "Write the release notes"); got.code != 0 {
			t.Fatalf("add: %d %s", got.code, got.errw)
		}
		if got := runCLI(t, root, "file", "--column", "doing", "fx-1", "acceptance_criterion", "Somebody has to settle this."); got.code != 0 {
			t.Fatalf("file: %d %s", got.code, got.errw)
		}
		return root
	}

	operatorRoot := build(t)
	operator := runCLI(t, operatorRoot, "move", "fx-1", "doing")
	if operator.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("the operator's move exited %d, wanted %d", operator.code, contract.ExitCode(contract.OutcomeRefused))
	}
	if name := refusalNameOf(operator.errw); name != contract.UnresolvedItem {
		t.Fatalf("the refusal name is %s, wanted %s", name, contract.UnresolvedItem)
	}
	if !strings.Contains(operator.errw, "--override") {
		t.Errorf("the operator's own refusal does not name --override:\n%s", operator.errw)
	}

	otherRoot := build(t)
	t.Setenv("DINAH_ACTOR", "sam")
	other := runCLI(t, otherRoot, "move", "fx-1", "doing")
	if other.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("the non-operator's move exited %d, wanted %d", other.code, contract.ExitCode(contract.OutcomeRefused))
	}
	if name := refusalNameOf(other.errw); name != contract.UnresolvedItem {
		t.Fatalf("the refusal name is %s, wanted %s", name, contract.UnresolvedItem)
	}
	if strings.Contains(other.errw, "--override") {
		t.Errorf("a non-operator's refusal names --override, which he cannot pass:\n%s", other.errw)
	}
}

// missingFieldDefinition requires git.branch before a card may enter Doing,
// which is a field every workbench declares, so no fixture has to declare
// one of its own for this to bite.
const missingFieldDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Missing field",
  "columns": [
    { "id": "g20000000001", "title": "Intake", "kind": "intake" },
    { "id": "g20000000002", "title": "Doing", "kind": "work", "require_fields": ["git.branch"] },
    { "id": "g20000000003", "title": "Done", "kind": "done" }
  ]
}`

// TestTheOperatorRefusalAtTheMissingFieldRowNamesOverride is dinah-571's own
// case for the third of the four remaining overridable refusals.
func TestTheOperatorRefusalAtTheMissingFieldRowNamesOverride(t *testing.T) {
	build := func(t *testing.T) string {
		t.Helper()
		root := newBenchFromDefinition(t, missingFieldDefinition)
		declareFieldsOn(t, root, "fields:\n  git.branch:\n    type: string\n    meaning: the branch the card's code lives on\n    on: [card]\n")
		if got := runCLI(t, root, "add", "Write the release notes"); got.code != 0 {
			t.Fatalf("add: %d %s", got.code, got.errw)
		}
		return root
	}

	operatorRoot := build(t)
	operator := runCLI(t, operatorRoot, "move", "fx-1", "doing")
	if operator.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("the operator's move exited %d, wanted %d", operator.code, contract.ExitCode(contract.OutcomeRefused))
	}
	if name := refusalNameOf(operator.errw); name != contract.MissingField {
		t.Fatalf("the refusal name is %s, wanted %s", name, contract.MissingField)
	}
	if !strings.Contains(operator.errw, "--override") {
		t.Errorf("the operator's own refusal does not name --override:\n%s", operator.errw)
	}

	otherRoot := build(t)
	t.Setenv("DINAH_ACTOR", "sam")
	other := runCLI(t, otherRoot, "move", "fx-1", "doing")
	if other.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("the non-operator's move exited %d, wanted %d", other.code, contract.ExitCode(contract.OutcomeRefused))
	}
	if name := refusalNameOf(other.errw); name != contract.MissingField {
		t.Fatalf("the refusal name is %s, wanted %s", name, contract.MissingField)
	}
	if strings.Contains(other.errw, "--override") {
		t.Errorf("a non-operator's refusal names --override, which he cannot pass:\n%s", other.errw)
	}
}

// loopLimitOverrideDefinition declares a loop_limit of 1 on Doing, so one
// regressive departure is free and a second is refused.
const loopLimitOverrideDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Loop limit override",
  "columns": [
    { "id": "g30000000001", "title": "Intake", "kind": "intake" },
    { "id": "g30000000002", "title": "Doing", "kind": "work", "loop_limit": 1 },
    { "id": "g30000000003", "title": "Review", "kind": "work" },
    { "id": "g30000000004", "title": "Done", "kind": "done" }
  ]
}`

// TestTheOperatorRefusalAtTheLoopLimitNamesOverride is dinah-571's own case
// for the fourth of the four remaining overridable refusals. The fixture
// spends the fixture's one free regressive departure first, on either actor,
// since RegressiveDepartures counts journal events rather than the actor who
// made them, and only the second regressive attempt reaches the refusal
// being pinned.
func TestTheOperatorRefusalAtTheLoopLimitNamesOverride(t *testing.T) {
	build := func(t *testing.T) string {
		t.Helper()
		root := newBenchFromDefinition(t, loopLimitOverrideDefinition)
		if got := runCLI(t, root, "add", "Write the release notes"); got.code != 0 {
			t.Fatalf("add: %d %s", got.code, got.errw)
		}
		if got := runCLI(t, root, "move", "fx-1", "doing"); got.code != 0 {
			t.Fatalf("move to doing: %d %s", got.code, got.errw)
		}
		if got := runCLI(t, root, "move", "fx-1", "intake"); got.code != 0 {
			t.Fatalf("spend the free regressive departure: %d %s", got.code, got.errw)
		}
		if got := runCLI(t, root, "move", "fx-1", "doing"); got.code != 0 {
			t.Fatalf("move back to doing: %d %s", got.code, got.errw)
		}
		return root
	}

	// at-loop-limit's un-conditioned .next fragment has always named
	// --override, since it has always told a non-operator to go ask the
	// operator to use it. What changes here is not whether the word appears
	// but who it is addressed to, so the two messages are told apart by
	// "yourself" (the operator's own fragment, which needs no asking) rather
	// than by the flag's name alone.
	operatorRoot := build(t)
	operator := runCLI(t, operatorRoot, "move", "fx-1", "intake")
	if operator.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("the operator's move exited %d, wanted %d", operator.code, contract.ExitCode(contract.OutcomeRefused))
	}
	if name := refusalNameOf(operator.errw); name != contract.AtLoopLimit {
		t.Fatalf("the refusal name is %s, wanted %s", name, contract.AtLoopLimit)
	}
	if !strings.Contains(operator.errw, "--override") || !strings.Contains(operator.errw, "yourself") {
		t.Errorf("the operator's own refusal does not tell him to carry the move through himself:\n%s", operator.errw)
	}
	if strings.Contains(operator.errw, "ask the operator") {
		t.Errorf("the operator's own refusal tells him to ask the operator:\n%s", operator.errw)
	}

	otherRoot := build(t)
	t.Setenv("DINAH_ACTOR", "sam")
	other := runCLI(t, otherRoot, "move", "fx-1", "intake")
	if other.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("the non-operator's move exited %d, wanted %d", other.code, contract.ExitCode(contract.OutcomeRefused))
	}
	if name := refusalNameOf(other.errw); name != contract.AtLoopLimit {
		t.Fatalf("the refusal name is %s, wanted %s", name, contract.AtLoopLimit)
	}
	if !strings.Contains(other.errw, "ask the operator") {
		t.Errorf("a non-operator's refusal does not tell him to ask the operator:\n%s", other.errw)
	}
	if strings.Contains(other.errw, "yourself") {
		t.Errorf("a non-operator's refusal tells him to carry the move through himself:\n%s", other.errw)
	}
}
