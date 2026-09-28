package bench

import (
	"os"
	"path/filepath"
	"strings"
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
// appendLocked appends one event under the lock of the journal's own entity,
// taking the lock for the one append and giving it back, which is what a test
// writing a line outside any verb has to do now that AppendEvent refuses an
// append made without that lock.
func appendLocked(t *testing.T, path string, ev Event) error {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("make the journal's directory: %v", err)
	}
	held, err := Acquire(filepath.Dir(path), "test", "2026-01-01T00:00:00Z")
	if err != nil {
		return err
	}
	defer held.Release()
	return AppendEvent(held, path, ev)
}

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
	if ev.Event != contract.EventMoved || ev.TS != "2026-01-01T00:00:03Z" || ev.Actor.Name != "sam" {
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

// TestAppendEventRefusesAnEmptyActor is dinah-540 AC-12. AppendEvent is the
// one function every journal write in this codebase funnels through, so this
// is the backstop test: whatever per-verb check might be missing or wrong
// above it, this function itself never writes a line with no actor name.
//
// The journal does not exist before the call, which is what lets the refused
// case assert the strong form of "before any byte is written": not merely
// that the journal is unchanged, but that it was never created.
func TestAppendEventRefusesAnEmptyActor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, JournalName)
	held, err := Acquire(dir, "alka", "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("take the directory's lock: %v", err)
	}
	defer held.Release()

	err = AppendEvent(held, path, Event{TS: "2026-01-01T00:00:00Z", Event: contract.EventCreated})
	refusal, ok := err.(*contract.Refusal)
	if !ok || refusal.Name != contract.NoOwner {
		t.Fatalf("an event with an empty actor name: wanted a no-owner refusal, got %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("the refused call created the journal, which is a byte written before the refusal")
	}

	if err := AppendEvent(held, path, Event{TS: "2026-01-01T00:00:01Z", Event: contract.EventCreated, Actor: NamedActor("alka")}); err != nil {
		t.Fatalf("a named actor: wanted the append to succeed, got %v", err)
	}
	events := readJournalForTest(t, path)
	if len(events) != 1 {
		t.Fatalf("wanted the one successful append to land, got %d events", len(events))
	}
	if events[0].Actor.Name != "alka" {
		t.Errorf("the appended event carries actor %q, wanted alka", events[0].Actor.Name)
	}
}

// TestAppendEventRefusesAnAppendItsLockDoesNotGuard is dinah-637/criteria/31's
// first half. An append made with no lock, with a lock already released, or
// with the lock of another entity's directory is refused
// dinah.journal-unlocked before the file is opened, so a journal that exists
// keeps its bytes and size and one that does not is never created. The
// accepting case sits beside the refusing ones, so a guard refusing every
// append does not pass.
func TestAppendEventRefusesAnAppendItsLockDoesNotGuard(t *testing.T) {
	root := t.TempDir()
	card := filepath.Join(root, "card")
	other := filepath.Join(root, "other")
	for _, dir := range []string{card, other} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("make %s: %v", dir, err)
		}
	}
	existing := filepath.Join(card, JournalName)
	absent := filepath.Join(other, JournalName)
	line := Event{TS: "2026-01-01T00:00:00Z", Event: contract.EventCommented, Actor: NamedActor("alka")}
	if err := appendLocked(t, existing, line); err != nil {
		t.Fatalf("seed the journal: %v", err)
	}
	before, err := os.ReadFile(existing)
	if err != nil {
		t.Fatalf("read the seeded journal: %v", err)
	}

	released, err := Acquire(card, "alka", "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("take the card's lock: %v", err)
	}
	released.Release()
	otherLock, err := Acquire(other, "alka", "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("take the other directory's lock: %v", err)
	}
	defer otherLock.Release()

	cases := []struct {
		name string
		held *Lock
		path string
	}{
		{"no lock at all", nil, existing},
		{"a released lock", released, existing},
		{"another entity's lock", otherLock, existing},
		{"no lock, on a journal that does not exist", nil, absent},
	}
	refused := 0
	for _, c := range cases {
		err := AppendEvent(c.held, c.path, line)
		refusal, ok := err.(*contract.Refusal)
		if !ok || refusal.Name != contract.JournalUnlocked || refusal.Detail != c.path {
			t.Errorf("%s: wanted dinah.journal-unlocked naming %s, got %v", c.name, c.path, err)
			continue
		}
		refused++
	}
	if refused != len(cases) {
		t.Fatalf("refused %d of %d unguarded appends", refused, len(cases))
	}
	after, err := os.ReadFile(existing)
	if err != nil {
		t.Fatalf("read the journal after the refusals: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("a refused append changed the journal: %d bytes before, %d after", len(before), len(after))
	}
	if _, err := os.Stat(absent); !os.IsNotExist(err) {
		t.Errorf("a refused append created %s", absent)
	}

	held, err := Acquire(card, "alka", "2026-01-01T00:00:01Z")
	if err != nil {
		t.Fatalf("take the card's lock again: %v", err)
	}
	defer held.Release()
	if err := AppendEvent(held, existing, line); err != nil {
		t.Fatalf("an append under the card's own lock: wanted it to land, got %v", err)
	}
	if got := len(readJournalForTest(t, existing)); got != 2 {
		t.Errorf("the guarded append left %d lines, wanted 2", got)
	}
}

// TestAnAppendQuarantinesATornTailAndKeepsAWholeLine drives the writer's two
// cases of dinah-637's section 3.5 on one journal each. A final fragment that
// does not decode is written, byte for byte, to a journal.torn.<stamp>
// sidecar, the journal is cut back to its last whole line, and a
// journal_tail_trimmed line carrying the fragment's byte count and the
// sidecar's name is appended ahead of the new line. A final line that decodes
// and only lacks its newline is kept byte for byte, gains the newline, and
// nothing is quarantined.
func TestAnAppendQuarantinesATornTailAndKeepsAWholeLine(t *testing.T) {
	first := `{"ts":"2026-01-01T00:00:00Z","event":"commented","actor":{"name":"alka"},"comment":"aaaaaaaaaaaa"}`
	next := Event{TS: "2026-01-02T03:04:05Z", Event: contract.EventCommented, Actor: NamedActor("alka"), Comment: "bbbbbbbbbbbb"}

	t.Run("a torn tail is quarantined", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, JournalName)
		fragment := `{"ts":"2026-01-01T00:00:01Z","event":"comm`
		if err := os.WriteFile(path, []byte(first+"\n"+fragment), 0o644); err != nil {
			t.Fatalf("plant: %v", err)
		}
		if err := appendLocked(t, path, next); err != nil {
			t.Fatalf("append: %v", err)
		}
		sidecars := TornSidecars(dir)
		if len(sidecars) != 1 {
			t.Fatalf("wanted one sidecar, got %v", sidecars)
		}
		if filepath.Base(sidecars[0]) != TornSidecarPrefix+"20260102T030405Z" {
			t.Errorf("the sidecar is named %s, wanted it stamped with the appending line's time", filepath.Base(sidecars[0]))
		}
		kept, err := os.ReadFile(sidecars[0])
		if err != nil {
			t.Fatalf("read the sidecar: %v", err)
		}
		if string(kept) != fragment {
			t.Errorf("the sidecar holds %q, wanted the fragment %q byte for byte", kept, fragment)
		}
		events := readJournalForTest(t, path)
		if len(events) != 3 {
			t.Fatalf("wanted the first line, the trim and the new line, got %d lines", len(events))
		}
		trim := events[1]
		if trim.Event != contract.EventJournalTailTrimmed || trim.Trimmed != len(fragment) || trim.Note != filepath.Base(sidecars[0]) {
			t.Errorf("the trim line is %+v, wanted journal_tail_trimmed carrying %d bytes and naming the sidecar", trim, len(fragment))
		}
		if events[2].Comment != next.Comment {
			t.Errorf("the appending line did not land last: %+v", events[2])
		}
		text, _ := os.ReadFile(path)
		if !strings.HasPrefix(string(text), first+"\n") {
			t.Errorf("the whole line before the fragment did not survive byte for byte")
		}
	})

	t.Run("a whole final line lacking its newline is kept", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, JournalName)
		if err := os.WriteFile(path, []byte(first), 0o644); err != nil {
			t.Fatalf("plant: %v", err)
		}
		if got := readJournalForTest(t, path); len(got) != 1 {
			t.Fatalf("a read before the append answered %d lines, wanted the whole final line", len(got))
		}
		if err := appendLocked(t, path, next); err != nil {
			t.Fatalf("append: %v", err)
		}
		if sidecars := TornSidecars(dir); len(sidecars) != 0 {
			t.Errorf("a whole line was quarantined: %v", sidecars)
		}
		text, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		line, err := EncodeEvent(next)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		if want := first + "\n" + string(line) + "\n"; string(text) != want {
			t.Errorf("the journal reads %q, wanted the first line byte for byte, a newline, and the new line", text)
		}
		for _, ev := range readJournalForTest(t, path) {
			if ev.Event == contract.EventJournalTailTrimmed {
				t.Errorf("a journal_tail_trimmed line was written for a whole line")
			}
		}
	})
}

// TestADamagedLineThatIsNotLastRefusesTheRead is the reading half of section
// 3.5: a line that does not decode and is not the last refuses the read
// dinah.journal-unreadable naming the file and the one-based line number,
// while the same bytes as the final line with no newline are a torn tail the
// read skips.
func TestADamagedLineThatIsNotLastRefusesTheRead(t *testing.T) {
	whole := `{"ts":"2026-01-01T00:00:00Z","event":"created","actor":{"name":"alka"}}`
	damaged := `{"ts":"2026-01-01T00:00:01Z","ev`
	path := filepath.Join(t.TempDir(), JournalName)
	if err := os.WriteFile(path, []byte(whole+"\n"+damaged+"\n"+whole+"\n"), 0o644); err != nil {
		t.Fatalf("plant: %v", err)
	}
	_, _, err := ReadJournal(path)
	refusal, ok := err.(*contract.Refusal)
	if !ok || refusal.Name != contract.JournalUnreadable || refusal.Detail != path+":2" {
		t.Fatalf("wanted dinah.journal-unreadable naming %s:2, got %v", path, err)
	}
	if err := os.WriteFile(path, []byte(whole+"\n"+damaged), 0o644); err != nil {
		t.Fatalf("plant the torn form: %v", err)
	}
	events, torn, err := ReadJournal(path)
	if err != nil || !torn || len(events) != 1 {
		t.Errorf("the same bytes as a final line with no newline: wanted one event and a torn report, got %d, %v, %v", len(events), torn, err)
	}
}

// TestAJournalLineCarriesMarkupAsItself asserts that a line is encoded with
// HTML escaping off, so <, > and & stand in the file as themselves and the
// journal stays readable with grep, and that the line still decodes to the
// same text.
func TestAJournalLineCarriesMarkupAsItself(t *testing.T) {
	path := filepath.Join(t.TempDir(), JournalName)
	text := "a <b> & c"
	if err := appendLocked(t, path, Event{TS: "2026-01-01T00:00:00Z", Event: contract.EventCommented, Actor: NamedActor("alka"), Text: text}); err != nil {
		t.Fatalf("append: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(raw), `"text":"a <b> & c"`) || strings.Contains(string(raw), `\u003c`) {
		t.Errorf("the line reads %s, wanted the markup as itself", raw)
	}
	if got := readJournalForTest(t, path); got[0].Text != text {
		t.Errorf("the line decodes to %q, wanted %q", got[0].Text, text)
	}
}
