package verb

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// memberCounts reads a card's live comments and items from its record.
func (h *harness) memberCounts(ref string) (comments, items int, err error) {
	record, err := h.library.Bench.LoadCardRecord(h.card(ref))
	if err != nil {
		return 0, 0, err
	}
	return len(record.CommentsOf("", bench.LiveHalf)), len(record.ItemsIn(bench.LiveHalf, "")), nil
}

// TestATornTailOnACardUnitCardIsQuarantinedByTheNextWrite drives
// dinah-637/criteria/25. A card-unit card whose journal ends in a fragment
// that does not decode still answers every earlier member; the next comment
// succeeds, the fragment sits byte for byte in a journal.torn.<stamp>
// sidecar, the journal carries a journal_tail_trimmed line counting the
// fragment's bytes and naming the sidecar ahead of the commented line, every
// member is answered, and check reports the sidecar. A line that does not
// decode and is not last refuses the card's read dinah.journal-unreadable
// naming the file and the line, and check reports it.
//
// Arming: making ReadJournal refuse a torn final line as damage refuses the
// first read, which the member count assertion catches.
func TestATornTailOnACardUnitCardIsQuarantinedByTheNextWrite(t *testing.T) {
	h := newCardUnitHarness(t)
	card := h.add("A card whose journal was torn")
	h.comment(card, "The comment written before the crash.")
	h.file(card, "decision", "The decision filed before the crash.")
	journal := h.card(card).JournalPath()
	const fragment = `{"ts":"2026-08-17T09:00:00Z","event":"comm`
	handle, err := os.OpenFile(journal, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open the journal: %v", err)
	}
	if _, err := handle.WriteString(fragment); err != nil {
		t.Fatalf("plant the fragment: %v", err)
	}
	handle.Close()
	h.reopen()

	if comments, items, err := h.memberCounts(card); err != nil || comments != 1 || items != 1 {
		t.Fatalf("the torn card reads %d comments and %d items (%v), wanted every earlier member", comments, items, err)
	}
	h.comment(card, "The comment written after the crash.")
	sidecars := bench.TornSidecars(filepath.Dir(journal))
	if len(sidecars) != 1 {
		t.Fatalf("wanted one sidecar beside the journal, got %v", sidecars)
	}
	if kept, _ := os.ReadFile(sidecars[0]); string(kept) != fragment {
		t.Errorf("the sidecar holds %q, wanted the fragment byte for byte", kept)
	}
	events := h.events(card)
	var trim, after bench.Event
	for i, ev := range events {
		if ev.Event == contract.EventJournalTailTrimmed && i+1 < len(events) {
			trim, after = ev, events[i+1]
		}
	}
	if trim.Trimmed != len(fragment) || trim.Note != filepath.Base(sidecars[0]) || after.Event != contract.EventCommented {
		t.Errorf("the trim line reads %+v followed by %s, wanted %d bytes naming the sidecar and then the comment", trim, after.Event, len(fragment))
	}
	if comments, items, err := h.memberCounts(card); err != nil || comments != 2 || items != 1 {
		t.Errorf("after the comment the card reads %d comments and %d items (%v), wanted two and one", comments, items, err)
	}
	if detail, found := finding(h.check(), bench.FindingJournalTornQuarantined); !found || !strings.Contains(detail, filepath.Base(sidecars[0])) {
		t.Errorf("check reports the sidecar as %q (found %v), wanted it named", detail, found)
	}

	// A damaged line that is not the last one.
	data, err := os.ReadFile(journal)
	if err != nil {
		t.Fatalf("read the journal: %v", err)
	}
	lines := bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n"))
	lines[1] = []byte(`{"ts":"2026-08-17T09:00:00Z","event":`)
	if err := os.WriteFile(journal, append(bytes.Join(lines, []byte("\n")), '\n'), 0o644); err != nil {
		t.Fatalf("plant the damage: %v", err)
	}
	h.reopen()
	_, _, _, _, shown := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: card})
	var refusal *contract.Refusal
	if !errors.As(shown, &refusal) || refusal.Name != contract.JournalUnreadable || !strings.HasSuffix(refusal.Detail, "journal.ndjson:2") {
		t.Errorf("show of the damaged card answered %v, wanted %s naming the journal and line 2", shown, contract.JournalUnreadable)
	}
	found := false
	for _, f := range h.check() {
		if f.Key == bench.FindingJournalUnreadable && bench.SeverityOf(f) == bench.SeverityDefect {
			found = true
		}
	}
	if !found {
		t.Error("check does not report the damaged line at defect severity")
	}
}

// TestAFinalObjectThatIsNotALineIsQuarantined answers dinah-637/questions/28.
// A final line with no newline after it that is a JSON object but not a
// journal line, here a hand-edited commented line whose ordinal is a string,
// is what the reader skips as torn, so the writer takes it for a torn tail too:
// the next comment moves it to a sidecar, answers ok, and the card reads its
// earlier comment and the new one. Were the writer to keep it, the line would
// stop being the last one, and every later read of the card would be refused.
//
// Arming: deciding a tail is whole when it decodes as any JSON object, as
// decodesAsObject did, keeps the line; the comment is then answered refused
// dinah.journal-unreadable, which the first assertion reports.
func TestAFinalObjectThatIsNotALineIsQuarantined(t *testing.T) {
	h := newCardUnitHarness(t)
	card := h.add("A card whose last line was edited into a shape no line has")
	h.comment(card, "The comment written before the edit.")
	journal := h.card(card).JournalPath()
	const edited = `{"ts":"2026-08-17T09:00:00Z","event":"commented","actor":{"name":"alka"},"comment":"0123456789ab","ordinal":"2","text":"an edit"}`
	handle, err := os.OpenFile(journal, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open the journal: %v", err)
	}
	if _, err := handle.WriteString(edited); err != nil {
		t.Fatalf("plant the edited line: %v", err)
	}
	handle.Close()
	h.reopen()

	response := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: card, Text: "The comment written after the edit."})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("the comment after the edit answered %s %s naming %s, wanted ok", response.Outcome, response.Refusal, response.Detail)
	}
	h.reopen()
	sidecars := bench.TornSidecars(filepath.Dir(journal))
	if len(sidecars) != 1 {
		t.Fatalf("wanted the edited line in one sidecar beside the journal, got %v", sidecars)
	}
	if kept, _ := os.ReadFile(sidecars[0]); string(kept) != edited {
		t.Errorf("the sidecar holds %q, wanted the edited line byte for byte", kept)
	}
	if comments, _, err := h.memberCounts(card); err != nil || comments != 2 {
		t.Errorf("after the comment the card reads %d comments (%v), wanted the one before the edit and the new one", comments, err)
	}
}

// TestAWholeFinalLineWithoutItsNewlineIsKept drives dinah-637/criteria/30. A
// card-unit journal whose final line is a complete commented line with its
// newline removed answers that comment; the next comment writes a newline and
// its own line after the original bytes, and no sidecar and no trim line
// appear.
//
// Arming: quarantining every final line that lacks a newline leaves a
// sidecar behind, which the first assertion after the comment catches.
func TestAWholeFinalLineWithoutItsNewlineIsKept(t *testing.T) {
	h := newCardUnitHarness(t)
	card := h.add("A card whose journal lost its last newline")
	h.comment(card, "The comment whose line lost its newline.")
	journal := h.card(card).JournalPath()
	data, err := os.ReadFile(journal)
	if err != nil {
		t.Fatalf("read the journal: %v", err)
	}
	trimmed := bytes.TrimSuffix(data, []byte("\n"))
	if err := os.WriteFile(journal, trimmed, 0o644); err != nil {
		t.Fatalf("remove the newline: %v", err)
	}
	h.reopen()
	if comments, _, err := h.memberCounts(card); err != nil || comments != 1 {
		t.Fatalf("the card reads %d comments (%v), wanted the one whose line lost its newline", comments, err)
	}
	h.comment(card, "The next comment.")
	if sidecars := bench.TornSidecars(filepath.Dir(journal)); len(sidecars) != 0 {
		t.Errorf("a whole line was quarantined: %v", sidecars)
	}
	after, err := os.ReadFile(journal)
	if err != nil {
		t.Fatalf("read the journal: %v", err)
	}
	if !bytes.HasPrefix(after, append(append([]byte{}, trimmed...), '\n')) {
		t.Error("the original bytes were not kept, followed by a newline")
	}
	for _, ev := range h.events(card) {
		if ev.Event == contract.EventJournalTailTrimmed {
			t.Error("a trim line was written for a whole line")
		}
	}
	if comments, _, err := h.memberCounts(card); err != nil || comments != 2 {
		t.Errorf("after the next comment the card reads %d comments (%v), wanted both", comments, err)
	}
}
