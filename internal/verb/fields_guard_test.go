package verb

import (
	"path/filepath"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// fieldGuardDefinition declares both level axes a guarded write resolves
// against, and a column carrying a tier default so a relative expression has
// something to count from.
const fieldGuardDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Guarding",
  "levels": { "severity": ["trivial", "minor", "major"], "priority": ["later", "soon", "now"], "tier": ["workhorse", "frontier", "apex"] },
  "columns": [
    { "id": "d00000000001", "title": "Intake", "kind": "intake" },
    { "id": "d00000000002", "title": "Doing", "kind": "work", "tier": "workhorse" },
    { "id": "d00000000003", "title": "Finished", "kind": "done" }
  ]
}`

// TestEveryGuardedFieldRefusesAndAcceptsAtItsOwnGate runs one subtest per
// declared guard, and each runs a refused write and an accepted write against
// the same entity.
//
// The accepting half is what stops the whole file passing against a build that
// refuses everything, which is the shape a refusal-only sweep cannot tell from
// a working one. Each assertion reads the refusal name alone rather than the
// sentence, so a reworded catalog entry cannot redden it.
func TestEveryGuardedFieldRefusesAndAcceptsAtItsOwnGate(t *testing.T) {
	subtests := map[string]func(*testing.T){
		bench.GuardSlug:      guardSlug,
		bench.GuardLevel:     guardLevel,
		bench.GuardTier:      guardTier,
		bench.GuardState:     guardState,
		bench.GuardFilename:  guardFilename,
		bench.GuardKind:      guardColumnKind,
		bench.GuardCapacity:  guardCapacity,
		bench.GuardHold:      guardHold,
		bench.GuardColumnRef: guardColumnRef,
	}
	if len(subtests) != len(bench.Guards) {
		t.Fatalf("this file runs %d subtests and the closed guard set declares %d, so a guard is unexercised", len(subtests), len(bench.Guards))
	}
	for _, guard := range bench.Guards {
		body, covered := subtests[guard]
		if !covered {
			t.Fatalf("the guard %s is declared and this file runs no subtest for it", guard)
		}
		t.Run(guard, body)
	}
}

// guardSlug asserts both halves of a slug write: the grammar, and the
// confirmation a well-formed change still needs.
func guardSlug(t *testing.T) {
	h := harnessFromDefinition(t, "gd", fieldGuardDefinition)
	malformed := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: bench.WorkbenchRef,
		Field: bench.SlugField, Value: "Not A Slug", Confirm: true,
	})
	refusedWith(t, "a malformed slug", malformed, contract.Malformed)

	unconfirmed := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: bench.WorkbenchRef,
		Field: bench.SlugField, Value: "gd-dev",
	})
	refusedWith(t, "a well-formed slug carrying no confirmation", unconfirmed, contract.Unconfirmed)

	accepted := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: bench.WorkbenchRef,
		Field: bench.SlugField, Value: "gd-dev", Confirm: true,
	})
	acceptedOK(t, "a confirmed slug change", accepted)
	h.reopen()
	if got := h.library.Bench.Slug; got != "gd-dev" {
		t.Errorf("the workbench reads back the slug %q, wanted gd-dev", got)
	}
	// The reference the new slug composes resolves, which is the side effect
	// the confirmation exists to warn about.
	ref := h.add("a card reached by the new slug")
	if _, err := h.library.Bench.ResolveEntity(ref); err != nil {
		t.Errorf("the card %s does not resolve under the new slug: %v", ref, err)
	}
}

// guardLevel asserts a card's severity against the declared set.
func guardLevel(t *testing.T) {
	h := harnessFromDefinition(t, "gd", fieldGuardDefinition)
	ref := h.add("a card to classify")
	refused := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: ref, Field: bench.SeverityField, Value: "urgent",
	})
	refusedWith(t, "a severity the workbench does not declare", refused, contract.UnknownLevel)

	accepted := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: ref, Field: bench.SeverityField, Value: "major",
	})
	acceptedOK(t, "a declared severity", accepted)
	h.reopen()
	if got := h.card(ref).LevelOf(bench.SeverityField); got != "major" {
		t.Errorf("the card reads back the severity %q, wanted major", got)
	}
}

// guardTier asserts the relative expression a card's tier accepts beside a
// column, and the event that write records.
func guardTier(t *testing.T) {
	h := harnessFromDefinition(t, "gd", fieldGuardDefinition)
	ref := h.add("a card to raise at one station")
	refused := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: ref, Field: bench.TierField,
		Value: "+1", At: "d00000000001",
	})
	refusedWith(t, "a relative write at a column carrying no default", refused, contract.NoTierDefault)

	before := len(h.events(ref))
	accepted := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: ref, Field: bench.TierField,
		Value: "+1", At: "d00000000002",
	})
	acceptedOK(t, "a relative write at a column carrying a default", accepted)
	h.reopen()
	lines := h.events(ref)
	if len(lines) != before+1 {
		t.Fatalf("the card's journal grew by %d lines, wanted one", len(lines)-before)
	}
	if line := lines[len(lines)-1]; line.Event != contract.EventTierOverridden {
		t.Errorf("the write recorded %s, wanted %s", line.Event, contract.EventTierOverridden)
	}
}

// guardState asserts that a state write runs the checklist verb's own rules
// and lands the state that verb lands.
func guardState(t *testing.T) {
	h := harnessFromDefinition(t, "gd", fieldGuardDefinition)
	ref := h.add("a card carrying a checklist")
	fileItem(t, h, ref, "open_question", "does the deadline move?")
	fileItem(t, h, ref, "acceptance_criterion", "the endpoint answers 404 for an unknown id")
	h.reopen()

	unknown := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: ref + "/questions/1",
		Field: bench.ItemStateField, Value: "frobnicate", Note: "a note",
	})
	refusedWith(t, "a state outside the four", unknown, contract.UnknownValue)

	wrongKind := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: ref + "/questions/1",
		Field: bench.ItemStateField, Value: bench.ItemVerified, Note: "a note",
	})
	refusedWith(t, "verified on an open question", wrongKind, contract.WrongItemKind)

	accepted := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: ref + "/questions/1",
		Field: bench.ItemStateField, Value: bench.ItemResolved,
		Note: "the operator confirmed the deadline is the fifteenth",
	})
	acceptedOK(t, "resolved on an open question", accepted)
	h.reopen()
	value, err := h.library.GetField(&Request{
		Verb: "get", Ref: ref + "/questions/1", Field: bench.ItemStateField,
	})
	if err != nil {
		t.Fatalf("read the state back: %v", err)
	}
	if value != bench.ItemResolved {
		t.Errorf("the item reads back the state %q, wanted %s", value, bench.ItemResolved)
	}
	// The act is the verb's, so the journal line is the verb's own event
	// rather than the generic item write's.
	lines := h.events(ref)
	if line := lines[len(lines)-1]; line.Event != contract.EventItemResolved {
		t.Errorf("the write recorded %s, wanted %s", line.Event, contract.EventItemResolved)
	}
}

// guardFilename asserts that an attachment's filename routes through Rename,
// which is the arm that moves the payload with the name. The payload is what
// the assertion reads, because a plain frontmatter write of the new name
// succeeds and leaves the bytes behind under the old one.
func guardFilename(t *testing.T) {
	h := harnessFromDefinition(t, "gd", fieldGuardDefinition)
	ref := h.add("a card carrying a file")
	h.attach(ref, "notes.txt", "some bytes")
	h.reopen()

	refused := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: ref + "/attachments/1",
		Field: bench.FilenameField, Value: "not/a/name.txt",
	})
	refusedWith(t, "a name the format does not admit", refused, contract.Malformed)

	accepted := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: ref + "/attachments/1",
		Field: bench.FilenameField, Value: "renamed.txt",
	})
	acceptedOK(t, "a legal name", accepted)
	h.reopen()
	entity, err := h.library.Bench.ResolveEntity(ref + "/attachments/1")
	if err != nil {
		t.Fatalf("resolve the attachment: %v", err)
	}
	moved := filepath.Join(entity.Dir, bench.PayloadDir, "renamed.txt")
	text, err := bench.ReadText(moved)
	if err != nil {
		t.Fatalf("the payload did not move with the name: %v", err)
	}
	if text != "some bytes" {
		t.Errorf("the payload reads %q, wanted the bytes the attachment was filed with", text)
	}
}

// guardColumnKind asserts a column's kind against the closed set the profile
// declares.
func guardColumnKind(t *testing.T) {
	h := harnessFromDefinition(t, "gd", fieldGuardDefinition)
	refused := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: "d00000000002",
		Field: bench.ColumnKindField, Value: "frobnicate",
	})
	refusedWith(t, "a kind outside the declared set", refused, contract.Malformed)

	accepted := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: "d00000000002",
		Field: bench.ColumnKindField, Value: contract.Kinds[0],
	})
	acceptedOK(t, "a kind the profile declares", accepted)
	h.reopen()
	if got := h.library.Bench.Columns[1].Kind; got != contract.Kinds[0] {
		t.Errorf("the column reads back the kind %q, wanted %s", got, contract.Kinds[0])
	}
}

// guardCapacity asserts a column's capacity against the rule its create path
// applies: an integer above zero.
func guardCapacity(t *testing.T) {
	h := harnessFromDefinition(t, "gd", fieldGuardDefinition)
	for _, value := range []string{"0", "x"} {
		refused := h.library.SetField(&Request{
			Verb: "set", Actor: "alka", Ref: "d00000000002",
			Field: bench.CapacityField, Value: value,
		})
		refusedWith(t, "the capacity "+value, refused, contract.Malformed)
	}
	accepted := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: "d00000000002",
		Field: bench.CapacityField, Value: "3",
	})
	acceptedOK(t, "a capacity above zero", accepted)
	h.reopen()
	if got := h.library.Bench.Columns[1].Capacity; got != 3 {
		t.Errorf("the column reads back the capacity %d, wanted 3", got)
	}
}

// guardHold asserts a column's hold against the two words a person types, and
// asserts that neither the storage spelling nor a near miss of the typed one
// is admitted. What reaches disk is checked through the reopened column rather
// than through the value that was written, since typed and stored differ here.
func guardHold(t *testing.T) {
	h := harnessFromDefinition(t, "gd", fieldGuardDefinition)
	for _, value := range []string{"maybe", "true", "On", "1"} {
		refused := h.library.SetField(&Request{
			Verb: "set", Actor: "alka", Ref: "d00000000002",
			Field: bench.HoldField, Value: value,
		})
		refusedWith(t, "the hold "+value, refused, contract.Malformed)
	}
	accepted := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: "d00000000002",
		Field: bench.HoldField, Value: bench.HoldOn,
	})
	acceptedOK(t, "a hold turned on", accepted)
	if accepted.Detail != bench.HoldOn {
		t.Errorf("the answer carries the detail %q, wanted the word that was typed, %q", accepted.Detail, bench.HoldOn)
	}
	h.reopen()
	if !h.library.Bench.Columns[1].GateItems {
		t.Error("the column reads back as not holding after the hold was turned on")
	}

	cleared := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: "d00000000002",
		Field: bench.HoldField, Value: bench.HoldOff,
	})
	acceptedOK(t, "a hold turned off", cleared)
	if cleared.Detail != bench.HoldOff {
		t.Errorf("the answer carries the detail %q, wanted the word that was typed, %q", cleared.Detail, bench.HoldOff)
	}
	h.reopen()
	if h.library.Bench.Columns[1].GateItems {
		t.Error("the column reads back as holding after the hold was turned off")
	}
}

// guardColumnRef asserts both halves of an item's column write: a spelling
// that names no column is refused, and a spelling that names one is accepted
// and stored as that column's identifier rather than as what was typed.
//
// The stored value is read back rather than the answer alone, because the
// whole point of the guard is which of the three legal spellings reaches
// disk: a gate compares an item's column against a column identifier by
// string equality, so a title stored as a title holds nothing.
func guardColumnRef(t *testing.T) {
	h := harnessFromDefinition(t, "gd", fieldGuardDefinition)
	ref := h.add("a card carrying a checklist")
	fileItem(t, h, ref, "decision", "which column answers this")
	h.reopen()
	item := ref + "/decisions/1"

	unknown := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: item,
		Field: bench.ItemColumnField, Value: "no-such-column",
	})
	refusedWith(t, "a column nothing resolves", unknown, contract.UnknownColumn)

	accepted := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: item,
		Field: bench.ItemColumnField, Value: "Doing",
	})
	acceptedOK(t, "a column named by its title", accepted)
	h.reopen()
	value, err := h.library.GetField(&Request{
		Verb: "get", Ref: item, Field: bench.ItemColumnField,
	})
	if err != nil {
		t.Fatalf("read the column back: %v", err)
	}
	if value != "d00000000002" {
		t.Errorf("the item reads back the column %q, wanted the identifier d00000000002", value)
	}

	cleared := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: item,
		Field: bench.ItemColumnField, Value: "",
	})
	acceptedOK(t, "a column cleared", cleared)
	h.reopen()
	value, err = h.library.GetField(&Request{
		Verb: "get", Ref: item, Field: bench.ItemColumnField,
	})
	if err != nil {
		t.Fatalf("read the cleared column back: %v", err)
	}
	if value != "" {
		t.Errorf("the item reads back the column %q after a clear, wanted nothing", value)
	}
}

// fileItem files one checklist item under a card.
func fileItem(t *testing.T, h *harness, ref, kind, text string) {
	t.Helper()
	filed := h.library.File(&Request{
		Verb: "file", Actor: "alka", Card: ref, Kind: kind, Text: text,
	})
	if filed.Outcome != contract.OutcomeOK {
		t.Fatalf("file a %s: %s %s", kind, filed.Outcome, filed.Refusal)
	}
	h.reopen()
}

// refusedWith asserts that a write was refused under one name, reading the
// name alone so a reworded sentence cannot redden it.
func refusedWith(t *testing.T, what string, response *Response, want string) {
	t.Helper()
	if response.Outcome != contract.OutcomeRefused {
		t.Errorf("%s: wanted %s, got %s", what, contract.OutcomeRefused, response.Outcome)
		return
	}
	if response.Refusal != want {
		t.Errorf("%s: wanted %s, got %s", what, want, response.Refusal)
	}
}

// acceptedOK asserts that a write succeeded, which is the half that stops a
// refusal sweep passing against a build that refuses everything.
func acceptedOK(t *testing.T, what string, response *Response) {
	t.Helper()
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("%s: wanted %s, got %s %s", what, contract.OutcomeOK, response.Outcome, response.Refusal)
	}
}
