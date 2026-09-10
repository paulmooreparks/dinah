package verb

import (
	"path/filepath"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// fieldJournalDefinition declares the level axes a card's own severity write
// resolves against, which the shared fixture deliberately does not.
const fieldJournalDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Journalling",
  "levels": { "severity": ["trivial", "minor", "major"] },
  "columns": [
    { "id": "c00000000001", "title": "Intake", "kind": "intake" },
    { "id": "c00000000002", "title": "Doing", "kind": "work" },
    { "id": "c00000000003", "title": "Finished", "kind": "done" }
  ]
}`

// TestAWriteRecordsTheWrittenEntitysOwnEvent asserts the rule the format
// document states: a write below a card is journalled with the written
// entity's own event, naming that entity, on the journal of the nearest
// enclosing entity that carries one.
//
// The identifier is asserted rather than the event name alone, because an
// implementation writing the right event name with the wrong subject in it
// passes a presence check and fails this.
func TestAWriteRecordsTheWrittenEntitysOwnEvent(t *testing.T) {
	t.Run("a comment below a card", func(t *testing.T) {
		h := harnessFromDefinition(t, "jr", fieldJournalDefinition)
		ref := h.add("a card to write below")
		h.comment(ref, "the first thought")
		h.reopen()
		before := len(h.events(ref))
		line := onlyNewLine(t, h, ref, before, &Request{
			Verb: "set", Actor: "alka", Ref: ref + "/comments/1",
			Field: bench.BodyField, Value: "the corrected thought",
		})
		if line.Event != contract.EventCommentUpdated {
			t.Errorf("the line names the event %s, wanted %s", line.Event, contract.EventCommentUpdated)
		}
		if want := identifierOf(t, h, ref+"/comments/1"); line.Note != want {
			t.Errorf("the line's note is %q, wanted the comment's own identifier %q", line.Note, want)
		}
		if line.Field != bench.BodyField {
			t.Errorf("the line's field is %q, wanted %s", line.Field, bench.BodyField)
		}
	})

	t.Run("a checklist item below a card", func(t *testing.T) {
		h := harnessFromDefinition(t, "jr", fieldJournalDefinition)
		ref := h.add("a card to write below")
		filed := h.library.File(&Request{
			Verb: "file", Actor: "alka", Card: ref,
			Kind: "open_question", Text: "does the deadline move?",
		})
		if filed.Outcome != contract.OutcomeOK {
			t.Fatalf("file: %s %s", filed.Outcome, filed.Refusal)
		}
		h.reopen()
		before := len(h.events(ref))
		line := onlyNewLine(t, h, ref, before, &Request{
			Verb: "set", Actor: "alka", Ref: ref + "/questions/1",
			Field: bench.ItemOwnerField, Value: "operator",
		})
		if line.Event != contract.EventItemUpdated {
			t.Errorf("the line names the event %s, wanted %s", line.Event, contract.EventItemUpdated)
		}
		if want := identifierOf(t, h, ref+"/questions/1"); line.Note != want {
			t.Errorf("the line's note is %q, wanted the item's own identifier %q", line.Note, want)
		}
		if line.Field != bench.ItemOwnerField {
			t.Errorf("the line's field is %q, wanted %s", line.Field, bench.ItemOwnerField)
		}
		if line.From != "" || line.To != "operator" {
			t.Errorf("the line reads from %q to %q, wanted an absent from and operator", line.From, line.To)
		}
	})

	t.Run("an attachment below a card", func(t *testing.T) {
		h := harnessFromDefinition(t, "jr", fieldJournalDefinition)
		ref := h.add("a card to write below")
		h.attach(ref, "notes.txt", "some bytes")
		h.reopen()
		before := len(h.events(ref))
		line := onlyNewLine(t, h, ref, before, &Request{
			Verb: "set", Actor: "alka", Ref: ref + "/attachments/1",
			Field: bench.DescriptionField, Value: "the notes I took",
		})
		if line.Event != contract.EventAttachmentUpdated {
			t.Errorf("the line names the event %s, wanted %s", line.Event, contract.EventAttachmentUpdated)
		}
		if want := identifierOf(t, h, ref+"/attachments/1"); line.Note != want {
			t.Errorf("the line's note is %q, wanted the attachment's own identifier %q", line.Note, want)
		}
		if line.Field != bench.DescriptionField {
			t.Errorf("the line's field is %q, wanted %s", line.Field, bench.DescriptionField)
		}
	})

	// The other half of the attachment row. An attachment below a column
	// hangs under no card, so its line lands on the workbench's journal, and
	// an implementation that always reached for the card's journal would have
	// passed the three subtests above and written nowhere on this one.
	t.Run("an attachment below a column", func(t *testing.T) {
		h := harnessFromDefinition(t, "jr", fieldJournalDefinition)
		card := h.add("a card whose journal must not grow")
		column := h.library.Bench.Columns[0]
		attached := h.library.Attach(&Request{
			Verb: "attach", Actor: "alka", Ref: column.ID,
			File: writeSource(t, "column-notes.txt", "some bytes"),
		})
		if attached.Outcome != contract.OutcomeOK {
			t.Fatalf("attach below a column: %s %s", attached.Outcome, attached.Refusal)
		}
		h.reopen()
		benchBefore := len(h.benchEvents())
		cardBefore := len(h.events(card))
		reference := column.ID + "/attachments/1"
		response := h.library.SetField(&Request{
			Verb: "set", Actor: "alka", Ref: reference,
			Field: bench.DescriptionField, Value: "the station's own notes",
		})
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("set below a column: %s %s", response.Outcome, response.Refusal)
		}
		h.reopen()
		benchLines := h.benchEvents()
		if len(benchLines) != benchBefore+1 {
			t.Fatalf("the workbench journal grew by %d lines, wanted one", len(benchLines)-benchBefore)
		}
		line := benchLines[len(benchLines)-1]
		if line.Event != contract.EventAttachmentUpdated {
			t.Errorf("the workbench line names the event %s, wanted %s", line.Event, contract.EventAttachmentUpdated)
		}
		if want := identifierOf(t, h, reference); line.Note != want {
			t.Errorf("the workbench line's note is %q, wanted the attachment's own identifier %q", line.Note, want)
		}
		if got := len(h.events(card)); got != cardBefore {
			t.Errorf("the card's journal grew by %d lines, and this write is not about a card", got-cardBefore)
		}
	})

	// A card's own field is the card's own event, which is the reading that
	// stops the three new names from being asked to answer two questions.
	t.Run("a card's own field", func(t *testing.T) {
		h := harnessFromDefinition(t, "jr", fieldJournalDefinition)
		ref := h.add("a card to classify")
		before := len(h.events(ref))
		line := onlyNewLine(t, h, ref, before, &Request{
			Verb: "set", Actor: "alka", Ref: ref,
			Field: bench.SeverityField, Value: "major",
		})
		if line.Event != contract.EventCardUpdated {
			t.Errorf("the line names the event %s, wanted %s", line.Event, contract.EventCardUpdated)
		}
		if line.Note != "" {
			t.Errorf("the line carries the note %q, and a card's own write names no subject beside itself", line.Note)
		}
		if line.Field != bench.SeverityField || line.To != "major" {
			t.Errorf("the line reads field %q to %q", line.Field, line.To)
		}
	})

	// A column's own field lands on the workbench journal and carries the
	// field beside the note, which is the shape reshape's own lines do not
	// have and the case no other subtest here covers.
	t.Run("a column's own field", func(t *testing.T) {
		h := harnessFromDefinition(t, "jr", fieldJournalDefinition)
		column := h.library.Bench.Columns[1]
		before := len(h.benchEvents())
		response := h.library.SetField(&Request{
			Verb: "set", Actor: "alka", Ref: column.ID,
			Field: bench.TitleField, Value: "Doing, renamed",
		})
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("set a column's title: %s %s", response.Outcome, response.Refusal)
		}
		h.reopen()
		lines := h.benchEvents()
		if len(lines) != before+1 {
			t.Fatalf("the workbench journal grew by %d lines, wanted one", len(lines)-before)
		}
		line := lines[len(lines)-1]
		if line.Event != contract.EventColumnUpdated {
			t.Errorf("the line names the event %s, wanted %s", line.Event, contract.EventColumnUpdated)
		}
		if line.Note != column.ID {
			t.Errorf("the line's note is %q, wanted the column's own identifier %q", line.Note, column.ID)
		}
		if line.Field != bench.TitleField {
			t.Errorf("the line's field is %q, wanted %s", line.Field, bench.TitleField)
		}
		if line.From != "Doing" || line.To != "Doing, renamed" {
			t.Errorf("the line reads from %q to %q", line.From, line.To)
		}
	})

	// An attachment's filename routes to Rename, so its record is the
	// rename's own event rather than any name this card mints, and the two
	// spellings of that act leave one shape in the journal.
	t.Run("an attachment's filename is a rename", func(t *testing.T) {
		h := harnessFromDefinition(t, "jr", fieldJournalDefinition)
		ref := h.add("a card carrying a file")
		h.attach(ref, "notes.txt", "some bytes")
		h.reopen()
		before := len(h.events(ref))
		line := onlyNewLine(t, h, ref, before, &Request{
			Verb: "set", Actor: "alka", Ref: ref + "/attachments/1",
			Field: bench.FilenameField, Value: "renamed.txt",
		})
		if line.Event != contract.EventAttachmentRenamed {
			t.Errorf("the line names the event %s, wanted %s", line.Event, contract.EventAttachmentRenamed)
		}
		if line.Filename != "renamed.txt" || line.From != "notes.txt" {
			t.Errorf("the line reads filename %q from %q", line.Filename, line.From)
		}
	})
}

// onlyNewLine runs one write and returns the single journal line it appended
// to a card, failing when it appended any other number.
func onlyNewLine(t *testing.T, h *harness, ref string, before int, req *Request) bench.Event {
	t.Helper()
	response := h.library.SetField(req)
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("set %s %s: %s %s", req.Ref, req.Field, response.Outcome, response.Refusal)
	}
	h.reopen()
	lines := h.events(ref)
	if len(lines) != before+1 {
		t.Fatalf("the card's journal grew by %d lines, wanted one", len(lines)-before)
	}
	return lines[len(lines)-1]
}

// identifierOf resolves a reference and returns the entity's own identifier,
// which is what a journal line names its subject by.
func identifierOf(t *testing.T, h *harness, ref string) string {
	t.Helper()
	entity, err := h.library.Bench.ResolveEntity(ref)
	if err != nil {
		t.Fatalf("resolve %s: %v", ref, err)
	}
	return entity.ID
}

// writeSource writes a file for an attach to take, and returns its path.
func writeSource(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := bench.WriteText(path, body); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
