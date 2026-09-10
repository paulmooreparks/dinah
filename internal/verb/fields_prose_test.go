package verb

import (
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// proseValue is the three-line value every prose write in this file sends. It
// carries a blank line as well as two filled ones, because a body is stored
// verbatim and a writer that trimmed or rejoined it would still pass a check
// asking only that the text arrived.
const proseValue = "The first line.\n\nThe third line.\n"

// TestProseTravelsToTheAnchorAndNotToTheJournal asserts the two halves of the
// prose rule at once: a prose field carries its line breaks into the anchor
// and puts no copy of itself in the journal, and a field that is not prose
// refuses a value carrying a line break at all.
//
// Both subject sets are derived from Field.Prose rather than written out, so a
// field changing sides moves the declaration and nothing else, and the run is
// fatal when either set is empty. An empty set is the shape this board keeps
// catching: a sweep over nothing passes for free and reads exactly like a
// sweep that found everything in order.
func TestProseTravelsToTheAnchorAndNotToTheJournal(t *testing.T) {
	prose, plain := 0, 0
	for _, kind := range bench.EntityKinds() {
		for _, name := range bench.FieldsOf(kind) {
			field, known := bench.FieldOf(kind, name)
			if !known {
				t.Fatalf("FieldsOf(%q) reports %s and FieldOf does not carry it", kind, name)
			}
			if field.Prose {
				prose++
				t.Run("prose: "+kind+"/"+name, func(t *testing.T) {
					proseWriteReachesTheAnchorAlone(t, kind, field)
				})
				continue
			}
			// A field routing to another verb is written by that verb, which
			// runs its own value rules, so the one-line rule is not this
			// command's to enforce there.
			if field.Guard == bench.GuardState || field.Guard == bench.GuardFilename {
				continue
			}
			plain++
			t.Run("one line: "+kind+"/"+name, func(t *testing.T) {
				plainWriteRefusesALineBreak(t, kind, field)
			})
		}
	}
	if prose == 0 {
		t.Fatal("no field declares itself prose, so the first half of this check read nothing")
	}
	if plain == 0 {
		t.Fatal("no field declares itself one-line, so the second half of this check read nothing")
	}
}

// proseWriteReachesTheAnchorAlone runs one prose field through a write and a
// read, and reads the journal line the write appended.
func proseWriteReachesTheAnchorAlone(t *testing.T, kind string, field bench.Field) {
	h := harnessFromDefinition(t, "pr", fieldGuardDefinition)
	ref, journal := proseSubject(t, h, kind)
	before := len(journal(h))

	written := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: ref, Field: field.Name, Value: proseValue,
	})
	if written.Outcome != contract.OutcomeOK {
		t.Fatalf("write %s on %s: %s %s", field.Name, kind, written.Outcome, written.Refusal)
	}
	h.reopen()

	read, err := h.library.GetField(&Request{Verb: "get", Ref: ref, Field: field.Name})
	if err != nil {
		t.Fatalf("read %s on %s back: %v", field.Name, kind, err)
	}
	if read != proseValue {
		t.Errorf("%s/%s reads back %q, wanted the three lines that were written", kind, field.Name, read)
	}
	if strings.Count(read, "\n") != strings.Count(proseValue, "\n") {
		t.Errorf("%s/%s reads back %d line breaks, wanted %d", kind, field.Name, strings.Count(read, "\n"), strings.Count(proseValue, "\n"))
	}

	lines := journal(h)
	if len(lines) != before+1 {
		t.Fatalf("%s/%s appended %d journal lines, wanted one", kind, field.Name, len(lines)-before)
	}
	line := lines[len(lines)-1]
	if line.Field != field.Name {
		t.Errorf("%s/%s journalled the field %q", kind, field.Name, line.Field)
	}
	if line.From != "" {
		t.Errorf("%s/%s put %q in the line's from, and a prose write journals the act rather than the prose", kind, field.Name, line.From)
	}
	if line.To != "" {
		t.Errorf("%s/%s put %q in the line's to, and a prose write journals the act rather than the prose", kind, field.Name, line.To)
	}
}

// plainWriteRefusesALineBreak asserts both directions of the one-line rule for
// one field: the value carrying a break is refused, and the same value with
// the break taken out is accepted.
//
// The accepting half is what stops this passing against a build that refuses
// every write, which is the shape a refusal-only check cannot tell from a
// working one.
func plainWriteRefusesALineBreak(t *testing.T, kind string, field bench.Field) {
	h := harnessFromDefinition(t, "pr", fieldGuardDefinition)
	ref, _ := proseSubject(t, h, kind)
	value := plainValueFor(field)

	refused := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: ref, Field: field.Name,
		Value: value + "\nand a second line",
	})
	if refused.Outcome != contract.OutcomeRefused || refused.Refusal != contract.Malformed {
		t.Fatalf("%s/%s took a value carrying a line break: %s %s", kind, field.Name, refused.Outcome, refused.Refusal)
	}
	if refused.Detail != field.Name {
		t.Errorf("%s/%s: the refusal names %q rather than the field", kind, field.Name, refused.Detail)
	}

	accepted := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: ref, Field: field.Name,
		Value: value, Confirm: true,
	})
	if accepted.Outcome != contract.OutcomeOK {
		t.Fatalf("%s/%s refused the same value with the break taken out: %s %s", kind, field.Name, accepted.Outcome, accepted.Refusal)
	}
}

// plainValueFor is a value the field's own guard admits, so the accepting half
// of the one-line check tests the line rule rather than the guard.
func plainValueFor(field bench.Field) string {
	switch field.Guard {
	case bench.GuardSlug:
		return "pr-dev"
	case bench.GuardLevel, bench.GuardTier:
		if field.Name == bench.TierField {
			return "frontier"
		}
		if field.Name == bench.PriorityField {
			return "now"
		}
		return "major"
	case bench.GuardKind:
		return contract.Kinds[0]
	case bench.GuardCapacity:
		return "3"
	}
	return "a written value"
}

// proseSubject files one entity of a kind and returns its reference and a
// reader for the journal its writes land on.
func proseSubject(t *testing.T, h *harness, kind string) (string, func(*harness) []bench.Event) {
	t.Helper()
	benchJournal := func(h *harness) []bench.Event { return h.benchEvents() }
	switch kind {
	case bench.KindWorkbench:
		return bench.WorkbenchRef, benchJournal
	case bench.KindColumn:
		return h.library.Bench.Columns[1].ID, benchJournal
	case bench.KindWorkstream:
		view := h.newWorkstream("Autumn release")
		return view.Ref, func(h *harness) []bench.Event {
			return workstreamEvents(t, h, view.Ref)
		}
	}
	card := h.add("a card to write below")
	cardJournal := func(h *harness) []bench.Event { return h.events(card) }
	switch kind {
	case bench.KindCard:
		return card, cardJournal
	case bench.KindComment:
		h.comment(card, "the first thought")
		h.reopen()
		return card + "/comments/1", cardJournal
	case bench.KindItem:
		fileItem(t, h, card, "open_question", "does the deadline move?")
		return card + "/questions/1", cardJournal
	case bench.KindAttachment:
		h.attach(card, "notes.txt", "some bytes")
		h.reopen()
		return card + "/attachments/1", cardJournal
	}
	t.Fatalf("the grammar names the kind %s and this file does not know how to file one", kind)
	return "", nil
}

// workstreamEvents reads one workstream's own journal, which is where a write
// to a workstream field lands.
func workstreamEvents(t *testing.T, h *harness, ref string) []bench.Event {
	t.Helper()
	entity, err := h.library.Bench.ResolveEntity(ref)
	if err != nil {
		t.Fatalf("resolve %s: %v", ref, err)
	}
	lines, _, err := bench.ReadJournal(h.library.journalFor(entity))
	if err != nil {
		t.Fatalf("read the workstream journal: %v", err)
	}
	return lines
}
