package bench

import (
	"path/filepath"
	"testing"

	"dinah/internal/contract"
)

// writeJournal puts the given lines in a journal of their own and answers with
// its path, so a test naming one deviant line names it and nothing else.
func writeJournal(t *testing.T, lines ...string) string {
	t.Helper()
	text := ""
	for _, line := range lines {
		text += line + "\n"
	}
	path := filepath.Join(t.TempDir(), JournalName)
	write(t, path, text)
	return path
}

// readJournalForTest reads a journal and fails the test on an error or a torn
// tail, since every case in this file writes whole lines and none of them is
// about the torn-tail rule.
func readJournalForTest(t *testing.T, path string) []Event {
	t.Helper()
	events, torn, err := ReadJournal(path)
	if err != nil {
		t.Fatalf("read the journal: %v", err)
	}
	if torn {
		t.Fatalf("the read reported a torn tail, and every line written here is whole")
	}
	return events
}

// knownLine is an ordinary created event, written alongside each deviant line
// so a test can tell tolerance apart from a read that returned nothing.
const knownLine = `{"ts":"2026-01-01T00:00:00Z","event":"created","actor":"sam","title":"a card","to":"b00000000001","to_title":"Ready"}`

// TestReadJournalToleratesAnUnrecognisedCoreEventName asserts dinah-286 AC-3. A
// line naming an event outside the closed set of core names is read, kept, and
// returned with its fields intact, because a build meeting a name an older or a
// newer build wrote preserves the line rather than refusing the journal.
func TestReadJournalToleratesAnUnrecognisedCoreEventName(t *testing.T) {
	unrecognised := `{"ts":"2026-01-01T00:00:01Z","event":"a_future_core_event","actor":"sam","note":"kept"}`
	events := readJournalForTest(t, writeJournal(t, knownLine, unrecognised))
	if len(events) != 2 {
		t.Fatalf("read %d events, want both lines", len(events))
	}
	if events[0].Event != contract.EventCreated || events[0].Title != "a card" {
		t.Errorf("the known line read back as %+v", events[0])
	}
	if events[1].Event != "a_future_core_event" {
		t.Errorf("the unrecognised line read back as event %q, want it unchanged", events[1].Event)
	}
	if events[1].Note != "kept" {
		t.Errorf("the unrecognised line read back with note %q, want its fields kept", events[1].Note)
	}
}

// TestReadJournalToleratesADottedExtensionEventName asserts dinah-286 AC-3. An
// extension kind declaring a journal of its own writes dotted event names, so a
// dotted name is legitimate whatever it says and the read carries it through.
func TestReadJournalToleratesADottedExtensionEventName(t *testing.T) {
	extension := `{"ts":"2026-01-01T00:00:02Z","event":"acme.spend","actor":"sam","note":"12 tokens"}`
	events := readJournalForTest(t, writeJournal(t, knownLine, extension))
	if len(events) != 2 {
		t.Fatalf("read %d events, want both lines", len(events))
	}
	if events[1].Event != "acme.spend" {
		t.Errorf("the extension line read back as event %q, want it unchanged", events[1].Event)
	}
	if events[1].Note != "12 tokens" {
		t.Errorf("the extension line read back with note %q, want its fields kept", events[1].Note)
	}
}

// TestReadJournalToleratesALineMissingADeclaredRequiredField asserts dinah-286
// AC-4. A moved line carrying only the universal skeleton omits all four
// cross-reference fields the schema calls always present, and the read returns
// it with those fields empty rather than refusing it, which is what lets a
// journal written before a field was required stay readable forever.
func TestReadJournalToleratesALineMissingADeclaredRequiredField(t *testing.T) {
	sparse := `{"ts":"2026-01-01T00:00:03Z","event":"moved","actor":"sam"}`
	events := readJournalForTest(t, writeJournal(t, sparse))
	if len(events) != 1 {
		t.Fatalf("read %d events, want the one sparse line", len(events))
	}
	ev := events[0]
	if ev.Event != contract.EventMoved || ev.TS != "2026-01-01T00:00:03Z" || ev.Actor != "sam" {
		t.Fatalf("the skeleton read back as %+v", ev)
	}
	absent := map[string]string{
		"from":       ev.From,
		"from_title": ev.FromTitle,
		"to":         ev.To,
		"to_title":   ev.ToTitle,
	}
	for name, got := range absent {
		if got != "" {
			t.Errorf("%s read back as %q, want the absent field empty", name, got)
		}
	}
}

// TestReadJournalToleratesAnUnknownKey asserts dinah-286 AC-5. A line carrying
// a key the Event struct declares no field for is read, and its known fields
// decode as usual, which is what makes adding a field to an event later safe.
func TestReadJournalToleratesAnUnknownKey(t *testing.T) {
	extra := `{"ts":"2026-01-01T00:00:04Z","event":"blocked","actor":"sam","reason":"a vendor has not answered","future_field":"x"}`
	events := readJournalForTest(t, writeJournal(t, extra))
	if len(events) != 1 {
		t.Fatalf("read %d events, want the one line", len(events))
	}
	ev := events[0]
	if ev.Event != contract.EventBlocked {
		t.Errorf("the line read back as event %q, want %q", ev.Event, contract.EventBlocked)
	}
	if ev.Reason != "a vendor has not answered" {
		t.Errorf("reason read back as %q, want the known fields decoded", ev.Reason)
	}
}

// TestManualCorrectionAndMovedShareTheSameDecodedStructFields asserts dinah-286
// AC-8. A manual_correction line and a moved line carrying the same four
// cross-reference values decode into the same struct fields.
//
// This is a decode-path regression test and nothing more. ReadJournal runs one
// generic unmarshal per line and branches on the event name nowhere, so the two
// lines decode alike by construction, and the test reddens only if event-name
// specific decoding is added to ReadJournal that treats them differently. It is
// not a guard against a future writer of manual_correction producing a shape
// that drifts from the one the format promises, and no test on this card can be
// that guard, because the writer does not exist yet. dinah-314 owns the writer,
// and an open question recorded on that card obliges it to bring its own test.
func TestManualCorrectionAndMovedShareTheSameDecodedStructFields(t *testing.T) {
	correction := `{"ts":"2026-01-01T00:00:05Z","event":"manual_correction","actor":"sam","from":"b00000000001","to":"b00000000002","from_title":"Ready","to_title":"Doing"}`
	moved := `{"ts":"2026-01-01T00:00:06Z","event":"moved","actor":"sam","from":"b00000000001","to":"b00000000002","from_title":"Ready","to_title":"Doing"}`
	events := readJournalForTest(t, writeJournal(t, correction, moved))
	if len(events) != 2 {
		t.Fatalf("read %d events, want both lines", len(events))
	}
	first, second := events[0], events[1]
	if first.Event != contract.EventManualCorrection || second.Event != contract.EventMoved {
		t.Fatalf("read events %q and %q, want manual_correction then moved", first.Event, second.Event)
	}
	if first.From != second.From || first.To != second.To {
		t.Errorf("from and to decoded as %q, %q and %q, %q, want one pair of fields", first.From, first.To, second.From, second.To)
	}
	if first.FromTitle != second.FromTitle || first.ToTitle != second.ToTitle {
		t.Errorf("from_title and to_title decoded as %q, %q and %q, %q, want one pair of fields", first.FromTitle, first.ToTitle, second.FromTitle, second.ToTitle)
	}
	if first.From != "b00000000001" || first.ToTitle != "Doing" {
		t.Errorf("the manual_correction line decoded as %+v, want the four values it carried", first)
	}
}

// TestEventRecordsRequiresTheEntityIdOnARestoredNote asserts dinah-286 AC-9.
// eventRecords recognises a restored line as the point of record of a restore
// only when the line's note carries the entity's own identifier, which is the
// requirement the format's restored row cites.
func TestEventRecordsRequiresTheEntityIdOnARestoredNote(t *testing.T) {
	const id = "c00000000001"
	matching := Event{Event: contract.EventRestored, Note: id}
	if !eventRecords(matching, OpRestore, id) {
		t.Errorf("a restored line noting %q did not record the restore of %q", matching.Note, id)
	}
	other := Event{Event: contract.EventRestored, Note: "c00000000009"}
	if eventRecords(other, OpRestore, id) {
		t.Errorf("a restored line noting %q recorded the restore of %q", other.Note, id)
	}
	unnoted := Event{Event: contract.EventRestored}
	if eventRecords(unnoted, OpRestore, id) {
		t.Errorf("a restored line carrying no note recorded the restore of %q", id)
	}
}

// TestEventRecordsTreatsArchivedAndRestoredAlike asserts dinah-286 AC-9. The
// two operations sit behind one note test in eventRecords, so they agree on a
// matching identifier and on a mismatched one, and this reddens if one branch
// is ever given a matching rule the other does not have.
func TestEventRecordsTreatsArchivedAndRestoredAlike(t *testing.T) {
	const id = "c00000000001"
	cases := []struct {
		name string
		note string
		want bool
	}{
		{name: "the entity's own identifier", note: id, want: true},
		{name: "another entity's identifier", note: "c00000000009", want: false},
	}
	for _, c := range cases {
		archived := Event{Event: contract.EventArchived, Note: c.note}
		restored := Event{Event: contract.EventRestored, Note: c.note}
		gotArchive := eventRecords(archived, OpArchive, id)
		gotRestore := eventRecords(restored, OpRestore, id)
		if gotArchive != c.want || gotRestore != c.want {
			t.Errorf("%s: archive answered %v and restore answered %v, want both %v", c.name, gotArchive, gotRestore, c.want)
		}
	}
}
