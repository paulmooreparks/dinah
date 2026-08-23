package verb

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// checkpoint runs one checkpoint and fails the test unless it answered.
func (h *harness) checkpoint(req *Request) *ChangeSet {
	h.t.Helper()
	req.Verb = "changes"
	set, err := h.library.Changes(req)
	if err != nil {
		h.t.Fatalf("changes: %v", err)
	}
	return set
}

// mint takes a fresh cursor, which is what every test below starts from
// because a cursor is the only way to say "from here".
//
// The clock is deliberately left where it is. The harness clock is frozen and
// the stored format is second resolution, so every act a fixture runs after
// minting lands inside the cursor's own second, which is exactly the case a
// cursor recorded as a single point in the merged order silently lost. An
// earlier form of this helper stepped a minute forward and said in as many
// words that it did so to keep fixtures clear of that comparison, and the
// defect it was hiding then survived two reviews and fifteen criteria. A
// fixture that wants a later stamp advances the clock itself and says why.
func (h *harness) mint() string {
	h.t.Helper()
	return h.checkpoint(&Request{}).Cursor
}

// archive moves an entity out of the live set and fails the test unless it
// went.
func (h *harness) archive(ref string) {
	h.t.Helper()
	response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: ref})
	if response.Outcome != contract.OutcomeOK {
		h.t.Fatalf("archive %s: %s %s", ref, response.Outcome, response.Refusal)
	}
	h.reopen()
}

// remove destroys an entity and fails the test unless it went.
func (h *harness) remove(ref string) {
	h.t.Helper()
	response := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: ref, Confirm: true})
	if response.Outcome != contract.OutcomeOK {
		h.t.Fatalf("delete %s: %s %s", ref, response.Outcome, response.Refusal)
	}
	h.reopen()
}

// workstream files a workstream and returns its identifier.
func (h *harness) workstream(title string) (id, ref string) {
	h.t.Helper()
	response := h.library.NewWorkstream(&Request{Verb: "workstream", Actor: "alka", Action: "new", Workstream: title})
	if response.Outcome != contract.OutcomeOK {
		h.t.Fatalf("workstream new %q: %s %s", title, response.Outcome, response.Refusal)
	}
	h.reopen()
	return response.Workstream.ID, bench.WorkstreamRefPrefix + response.Workstream.Ref
}

// cardID resolves a reference to the identifier the change walk keys on.
func (h *harness) cardID(ref string) string {
	h.t.Helper()
	return h.card(ref).ID
}

// journalOf is a live card's journal path, which several tests below damage on
// purpose.
func (h *harness) journalOf(ref string) string {
	h.t.Helper()
	return filepath.Join(h.library.Bench.CardsRoot(), h.cardID(ref), bench.JournalName)
}

// appendRaw writes one line to a journal exactly as given, which is how a test
// reaches the two shapes AppendEvent cannot write: a torn tail and a malformed
// line with a good line after it.
func (h *harness) appendRaw(path, line string) {
	h.t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		h.t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	if _, err := f.WriteString(line); err != nil {
		h.t.Fatalf("write %s: %v", path, err)
	}
}

// eventNames lists the acts an answer delivered, which is what most of the
// assertions below read.
func eventNames(set *ChangeSet) []string {
	names := make([]string, 0, len(set.Events))
	for _, ev := range set.Events {
		names = append(names, ev.Event.Event)
	}
	return names
}

// cardIDs lists the identifiers an answer put in cards.
func cardIDs(set *ChangeSet) []string {
	ids := make([]string, 0, len(set.Cards))
	for _, card := range set.Cards {
		ids = append(ids, card.ID)
	}
	return ids
}

// holds reports whether a list carries a value, which reads better at a call
// site than a loop does.
func holds(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

// TestAFirstCheckpointMintsACursorAndReportsNothing covers dinah-120 AC-1. The
// bench carries history before the call, so the empty arrays distinguish
// "reports nothing" from "there was nothing".
func TestAFirstCheckpointMintsACursorAndReportsNothing(t *testing.T) {
	h := newHarness(t)
	first := h.add("A card")
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: first, Holder: "alka"})
	h.comment(first, "a note the history carries")

	set := h.checkpoint(&Request{})
	if set.Changed {
		t.Error("a first call reported the board as changed, and there is nothing it could be changed against")
	}
	if len(set.Events) != 0 || len(set.Cards) != 0 || len(set.Gone) != 0 {
		t.Errorf("a first call replayed history: %d events, %d cards, %d gone", len(set.Events), len(set.Cards), len(set.Gone))
	}
	if set.Cursor == "" {
		t.Error("a first call minted no cursor, so the caller has nothing to ask with")
	}
}

// TestAnUnchangedBenchAnswersWithTheSameTokenCoversAC2 covers dinah-120 AC-2.
// The token comes back byte for byte, so a caller compares two answers without
// decoding either.
func TestAnUnchangedBenchAnswersWithTheSameTokenCoversAC2(t *testing.T) {
	h := newHarness(t)
	h.add("A card")
	minted := h.mint()

	set := h.checkpoint(&Request{Since: minted})
	if set.Changed {
		t.Error("an untouched workbench reported a change")
	}
	if set.Cursor != minted {
		t.Errorf("the cursor came back rewritten:\nwanted %q\ngot    %q", minted, set.Cursor)
	}
}

// TestAMoveIsReportedOnceAndThenNotAgain covers dinah-120 AC-3, which is the
// card's whole reason for existing expressed as a test.
func TestAMoveIsReportedOnceAndThenNotAgain(t *testing.T) {
	h := newHarness(t)
	ref := h.add("A card that moves")
	// A second card nothing touches, so "the card in cards" is an assertion
	// about which card rather than about how many there are on the board.
	h.add("A card that does not move")
	minted := h.mint()

	h.mustDo(&Request{Verb: Move, Actor: "alka", Card: ref, State: doing})

	set := h.checkpoint(&Request{Since: minted})
	if !set.Changed {
		t.Fatal("a move left the board reading unchanged")
	}
	if names := eventNames(set); len(names) != 1 || names[0] != contract.EventMoved {
		t.Fatalf("wanted the one moved event, got %v", names)
	}
	if len(set.Cards) != 1 || set.Cards[0].ID != h.cardID(ref) || set.Cards[0].State != doing {
		t.Fatalf("wanted the moved card in its new state, got %+v", set.Cards)
	}
	if set.Events[0].Scope != ScopeCard || set.Events[0].ID != h.cardID(ref) {
		t.Errorf("the event does not name the card it came from: %+v", set.Events[0])
	}

	again := h.checkpoint(&Request{Since: set.Cursor})
	if again.Changed || len(again.Events) != 0 {
		t.Errorf("the same move was reported twice: changed=%v, %v", again.Changed, eventNames(again))
	}

	// The board has to move again before the cursor's position is load
	// bearing. Until it does, an answer that reports nothing is the digest
	// terms agreeing, and a cursor that never advanced its position would
	// pass on that alone.
	h.advance(time.Minute)
	h.reopen()
	h.comment(ref, "a second act, so the terms move again")
	later := h.checkpoint(&Request{Since: again.Cursor})
	if names := eventNames(later); len(names) != 1 || names[0] != contract.EventCommented {
		t.Errorf("the cursor did not advance past the move, so it is delivered again: %v", names)
	}
}

// TestTwoEventsInOneSecondAreBothDeliveredOnce covers dinah-120 AC-4. The
// stored format is second resolution, so this is the case a bare timestamp
// cursor loses. The two lines are appended directly at one stamp rather than
// raced, so the fixture is the case rather than an attempt at it.
func TestTwoEventsInOneSecondAreBothDeliveredOnce(t *testing.T) {
	h := newHarness(t)
	first := h.add("The first card")
	second := h.add("The second card")
	minted := h.mint()

	stamp := bench.Stamp(h.clock.Add(time.Hour))
	for _, ref := range []string{first, second} {
		if err := bench.AppendEvent(h.journalOf(ref), bench.Event{TS: stamp, Event: contract.EventCommented, Actor: "alka"}); err != nil {
			t.Fatalf("append to %s: %v", ref, err)
		}
	}

	set := h.checkpoint(&Request{Since: minted})
	if len(set.Events) != 2 {
		t.Fatalf("wanted both lines of the shared second, got %d: %v", len(set.Events), eventNames(set))
	}
	order := []string{set.Events[0].ID, set.Events[1].ID}
	repeat := h.checkpoint(&Request{Since: minted})
	if len(repeat.Events) != 2 || repeat.Events[0].ID != order[0] || repeat.Events[1].ID != order[1] {
		t.Errorf("the order of a shared second is not stable across calls: %v then %v", order, []string{repeat.Events[0].ID, repeat.Events[1].ID})
	}
	// A third line moves the terms again, so the call below is answered from
	// the cursor's position rather than from the digest terms agreeing. The
	// clock steps past the hour the pair above was stamped at, since a line
	// written before the cursor's own position is not a line after it.
	h.advance(2 * time.Hour)
	h.reopen()
	h.comment(first, "a later act on the first card")
	after := h.checkpoint(&Request{Since: set.Cursor})
	if names := eventNames(after); len(names) != 1 || names[0] != contract.EventCommented {
		t.Errorf("a line of the shared second was delivered twice: %v", names)
	}

	// And a cursor inside the shared second, which the pair above does not
	// reach because both of its lines fall on the same side of the cursor.
	// The later act goes to whichever card's key sorts lower, which is the
	// side a rank comparison reads as already delivered.
	h.advance(3 * time.Hour)
	h.reopen()
	lower, higher := first, second
	if h.cardID(second) < h.cardID(first) {
		lower, higher = second, first
	}
	h.comment(higher, "the act the cursor is taken after")
	between := h.mint()
	h.comment(lower, "the act the cursor was taken before")
	inside := h.checkpoint(&Request{Since: between})
	if names := eventNames(inside); len(names) != 1 {
		t.Errorf("the act written into the cursor's own second was lost: %v", names)
	}
}

// TestAnAnchorEditedOutOfBandPutsTheCardInCards covers dinah-120 AC-5, the
// half journal size alone cannot see. It is what dinah edit produces, so the
// anchor is rewritten directly here rather than through a verb.
func TestAnAnchorEditedOutOfBandPutsTheCardInCards(t *testing.T) {
	h := newHarness(t)
	ref := h.add("A card somebody edits")
	// A second card nothing touches. It is here to be reported: an anchor
	// rewritten with no journal line is the one case the walk has no evidence
	// for at all, so dinah-120 D-13 answers it with a whole-board resync, and
	// this fixture says so out loud rather than hiding it behind a board of
	// one card. The narrow rule the review asked for is pinned by
	// TestAnActThatExplainsItselfDoesNotResyncTheWholeBoard.
	bystander := h.add("A card nobody edits")
	minted := h.mint()

	anchor := filepath.Join(h.library.Bench.CardsRoot(), h.cardID(ref), bench.CardAnchor)
	text, err := bench.ReadText(anchor)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	if err := bench.WriteText(anchor, text+"\nA paragraph somebody typed into the file.\n"); err != nil {
		t.Fatalf("rewrite the anchor: %v", err)
	}

	set := h.checkpoint(&Request{Since: minted})
	if !set.Changed {
		t.Fatal("an anchor rewritten out of band left the board reading unchanged")
	}
	if len(set.Events) != 0 {
		t.Errorf("an edit that appended no journal line delivered events: %v", eventNames(set))
	}
	if !holds(cardIDs(set), h.cardID(ref)) {
		t.Errorf("the edited card is not in cards: %v", cardIDs(set))
	}
	if !holds(cardIDs(set), h.cardID(bystander)) {
		t.Errorf("the unattributable edit did not resync the board, and D-13 says it does: %v", cardIDs(set))
	}
}

// TestTheAnswerReportsBirthsAndDeparturesAndExcludesOldOnes covers dinah-120
// AC-6. Four assertions on one walk, and the archive case is what separates
// this from a naive reading of which identifiers vanished from a listing.
func TestTheAnswerReportsBirthsAndDeparturesAndExcludesOldOnes(t *testing.T) {
	h := newHarness(t)
	old := h.add("A card archived long ago")
	oldID := h.cardID(old)
	h.archive(old)

	minted := h.mint()

	born := h.add("A card filed after the cursor")
	leaving := h.add("A card archived after the cursor")
	leavingID := h.cardID(leaving)
	leavingTitle := h.card(leaving).Title
	leavingRef := h.card(leaving).Ref(h.library.Bench.Slug)
	h.archive(leaving)

	set := h.checkpoint(&Request{Since: minted})
	if !holds(cardIDs(set), h.cardID(born)) {
		t.Errorf("the card filed after the cursor is not in cards: %v", cardIDs(set))
	}
	if !holds(eventNames(set), contract.EventCreated) {
		t.Errorf("the created event of the new card was not delivered: %v", eventNames(set))
	}
	var departed *GoneEntity
	for i := range set.Gone {
		if set.Gone[i].ID == leavingID {
			departed = &set.Gone[i]
		}
		if set.Gone[i].ID == oldID {
			t.Errorf("a card archived before the cursor was minted is reported as gone: %+v", set.Gone[i])
		}
	}
	if departed == nil {
		t.Fatalf("the archived card is not in gone: %+v", set.Gone)
	}
	if departed.Fate != FateArchived || departed.Kind != ScopeCard {
		t.Errorf("wanted fate archived and kind card, got fate %q kind %q", departed.Fate, departed.Kind)
	}
	if departed.Title != leavingTitle || departed.Ref != leavingRef {
		t.Errorf("the entry is not named from the mirror's anchor: wanted %q/%q, got %q/%q",
			leavingTitle, leavingRef, departed.Title, departed.Ref)
	}
	if holds(cardIDs(set), oldID) || holds(cardIDs(set), leavingID) {
		t.Errorf("an archived card was reported as live: %v", cardIDs(set))
	}
	for _, ev := range set.Events {
		if ev.ID == oldID {
			t.Errorf("an event of the old archive was delivered: %+v", ev)
		}
	}
}

// TestAWorkbenchScopedEventCarriesNoIdentifier covers dinah-120 AC-7, which is
// what proves the root journal is inside the watched set rather than only the
// card journals.
func TestAWorkbenchScopedEventCarriesNoIdentifier(t *testing.T) {
	h := newHarness(t)
	minted := h.mint()

	response := h.library.SetWorkbench(&Request{Verb: "workbench", Actor: "alka", Action: "set", Field: "title", Value: "A renamed workbench"})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("workbench set title: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()

	set := h.checkpoint(&Request{Since: minted})
	if len(set.Events) != 1 {
		t.Fatalf("wanted the one workbench event, got %v", eventNames(set))
	}
	if set.Events[0].Scope != ScopeWorkbench || set.Events[0].ID != "" {
		t.Errorf("wanted scope workbench and no identifier, got %+v", set.Events[0])
	}
}

// TestAFilteredCallStillAdvancesPastWhatItDidNotReport covers dinah-120 AC-8,
// which pins D-2. The cursor advancing is the load-bearing half: without it
// the caller would be told about that change on every call forever.
func TestAFilteredCallStillAdvancesPastWhatItDidNotReport(t *testing.T) {
	h := newHarness(t)
	ref, higher := h.sameSecondPair()
	// An act on the higher-sorting card just before the cursor, so the move
	// below is not the newest line of its own second. Without it, the move is
	// the only line there is and the fixture cannot tell a cursor that
	// advanced past what it declined to report from a move that was never
	// delivered to anybody at all: the second assertion below reads the same
	// either way.
	h.comment(higher, "an act before the cursor")
	minted := h.mint()

	h.mustDo(&Request{Verb: Move, Actor: "alka", Card: ref, State: doing})

	set := h.checkpoint(&Request{Since: minted, State: aftercareSlug})
	if !set.Changed {
		t.Error("the board moved and a filtered call reported it as unchanged")
	}
	if len(set.Events) != 0 || len(set.Cards) != 0 || len(set.Gone) != 0 {
		t.Errorf("a filter let through a change outside it: %d events, %d cards, %d gone", len(set.Events), len(set.Cards), len(set.Gone))
	}

	// A control from the same cursor with the filter off. It is what makes
	// the empty arrays above mean "the filter narrowed it" rather than "the
	// walk never saw it", and those two read identically without it. Nothing
	// this verb does writes, so the two calls are independent.
	control := h.checkpoint(&Request{Since: minted})
	if names := eventNames(control); len(names) != 1 || names[0] != contract.EventMoved {
		t.Fatalf("the move was never delivered to anybody, so the filtered answer above proves nothing: %v", names)
	}

	// The board moves again, so the call below reads the cursor's position
	// rather than two digest terms that happen to agree. The filter comes off
	// for that call, which is what makes the assertion about the cursor rather
	// than about the filter: a cursor that had not advanced would deliver the
	// move it declined to report the moment somebody asked without a filter.
	h.advance(time.Minute)
	h.reopen()
	inside := h.add("A card filed after the filtered call")

	after := h.checkpoint(&Request{Since: set.Cursor})
	for _, ev := range after.Events {
		if ev.ID == h.cardID(ref) {
			t.Errorf("the filtered call did not advance past what it did not report: %+v", ev)
		}
	}
	if !holds(cardIDs(after), h.cardID(inside)) {
		t.Errorf("the call after the filtered one lost the change that followed it: %v", cardIDs(after))
	}
}

// TestABadCursorIsRefusedRatherThanResynced covers dinah-120 AC-9. Both cases
// refuse, because a silent resync hides the caller's bug and loses the events
// it was owed.
func TestABadCursorIsRefusedRatherThanResynced(t *testing.T) {
	h := newHarness(t)
	h.add("A card")

	cases := []struct {
		name  string
		token string
	}{
		{name: "a token that is not a cursor at all", token: "not-a-cursor"},
		{name: "a cursor minted against another workbench", token: foreignCursor(t, h)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := h.library.Changes(&Request{Verb: "changes", Since: c.token})
			refusal, ok := err.(*contract.Refusal)
			if !ok {
				t.Fatalf("wanted a refusal, got %v", err)
			}
			if refusal.Name != contract.Malformed {
				t.Errorf("wanted %s, got %s", contract.Malformed, refusal.Name)
			}
			if refusal.Detail != c.token {
				t.Errorf("wanted the token as the detail, got %q", refusal.Detail)
			}
		})
	}

	// The check suite covers the same two refusals, so a reader of the
	// command's own page is told what a bad cursor does.
	names := map[string]bool{}
	for _, check := range Checks("changes") {
		names[check.Refusal] = true
	}
	for _, wanted := range []string{contract.Malformed, contract.UnknownCard, contract.UnknownState} {
		if !names[wanted] {
			t.Errorf("the changes check list does not carry %s", wanted)
		}
	}
}

// foreignCursor is a well-formed token naming a workbench this one is not.
func foreignCursor(t *testing.T, h *harness) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"v":         cursorVersion,
		"workbench": h.library.Bench.Slug + "-somewhere-else",
		"live":      "sha256:0",
		"archive":   "sha256:0",
	})
	if err != nil {
		t.Fatalf("compose a foreign cursor: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// TestATornOrShrunkenJournalDoesNotFailTheCall covers dinah-120 AC-10. The
// first half is reuse rather than reimplementation, which is the constraint
// the card set, and the second is what check's torn-line trim produces under a
// live cursor.
func TestATornOrShrunkenJournalDoesNotFailTheCall(t *testing.T) {
	t.Run("a torn final line is skipped", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("A card whose last line was torn")
		minted := h.mint()
		h.appendRaw(h.journalOf(ref), `{"ts":"2026-08-17T10:00:00Z","event":"comm`)

		set := h.checkpoint(&Request{Since: minted})
		if len(set.Unreadable) != 0 {
			t.Errorf("a torn tail was reported as unreadable, and ReadJournal tolerates it: %v", set.Unreadable)
		}
		if !holds(cardIDs(set), h.cardID(ref)) {
			t.Errorf("the card whose journal grew is not in cards: %v", cardIDs(set))
		}
	})

	t.Run("a shrunken journal resyncs the card", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("A card whose journal was trimmed")
		h.comment(ref, "a line the trim removes")
		minted := h.mint()

		journal := h.journalOf(ref)
		text, err := bench.ReadText(journal)
		if err != nil {
			t.Fatalf("read the journal: %v", err)
		}
		lines := bench.SplitLines(strings.TrimRight(text, "\n"))
		if err := bench.WriteText(journal, strings.Join(lines[:len(lines)-1], "\n")+"\n"); err != nil {
			t.Fatalf("trim the journal: %v", err)
		}

		set := h.checkpoint(&Request{Since: minted})
		if !set.Changed {
			t.Fatal("a journal that shrank left the board reading unchanged")
		}
		if len(set.Events) != 0 {
			t.Errorf("a shrunken journal replayed its events: %v", eventNames(set))
		}
		if !holds(cardIDs(set), h.cardID(ref)) {
			t.Errorf("the resynced card is not in cards: %v", cardIDs(set))
		}
	})
}

// TestACheckpointWritesNothing covers dinah-120 AC-11, and it pins D-5
// mechanically rather than by inspection: every file of the tree is
// fingerprinted on either side of a call.
func TestACheckpointWritesNothing(t *testing.T) {
	h := newHarness(t)
	ref := h.add("A card somebody is holding")
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: ref, Holder: "alka", Expires: time.Minute})
	h.add("A second card")
	minted := h.mint()
	h.mustDo(&Request{Verb: Move, Actor: "alka", Card: ref, State: doing})

	// The clock is past the lease, so a read that lapses claims the way the
	// other reads of the bench do would rewrite this card's anchor here.
	h.advance(time.Hour)
	h.reopen()

	before := treeFingerprint(t, h.root)
	set := h.checkpoint(&Request{Since: minted})
	if !set.Changed {
		t.Fatal("the fixture moved a card and the call reported nothing, so this proves nothing")
	}
	after := treeFingerprint(t, h.root)
	if before != after {
		t.Error("a checkpoint changed the workbench on disk, and it is specified to write nothing")
	}
}

// treeFingerprint is every path under a root and the bytes at each one,
// hashed, which is what makes "nothing was written" a mechanical assertion.
func treeFingerprint(t *testing.T, root string) string {
	t.Helper()
	var entries []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if info.IsDir() {
			entries = append(entries, "dir "+filepath.ToSlash(relative))
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		sum := sha256.Sum256(data)
		entries = append(entries, "file "+filepath.ToSlash(relative)+" "+hex.EncodeToString(sum[:]))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	sort.Strings(entries)
	whole := sha256.Sum256([]byte(strings.Join(entries, "\n")))
	return hex.EncodeToString(whole[:])
}

// TestAnUnreadableJournalDegradesOneEntityAndNotTheCall covers dinah-120
// AC-13, and it covers the case AC-10 does not: ReadJournal tolerates a bad
// line only when it is final, so a good line is written after the bad one.
//
// The workbench journal is run through the same case, because that is where
// deleted events live, so an unparseable one means gone cannot report a
// removal in that window and the caller has to be told which journal went
// unread.
func TestAnUnreadableJournalDegradesOneEntityAndNotTheCall(t *testing.T) {
	h := newHarness(t)
	damaged := h.add("A card whose history will not parse")
	damagedID := h.cardID(damaged)
	sound := h.add("A card whose history is sound")
	minted := h.mint()

	h.appendRaw(h.journalOf(damaged), "this line is not JSON\n")
	if err := bench.AppendEvent(h.journalOf(damaged), bench.Event{TS: bench.Stamp(h.clock.Add(time.Hour)), Event: contract.EventCommented, Actor: "alka"}); err != nil {
		t.Fatalf("append past the bad line: %v", err)
	}
	h.appendRaw(h.library.Bench.JournalPath(), "this line is not JSON either\n")
	if err := bench.AppendEvent(h.library.Bench.JournalPath(), bench.Event{TS: bench.Stamp(h.clock.Add(time.Hour)), Event: contract.EventWorkbenchUpdated, Actor: "alka", Field: "title"}); err != nil {
		t.Fatalf("append past the workbench journal's bad line: %v", err)
	}
	h.comment(sound, "a line the sound card carries")

	set := h.checkpoint(&Request{Since: minted})
	if !holds(set.Unreadable, bench.CardsDir+"/"+damagedID) {
		t.Errorf("the damaged card is not named in unreadable: %v", set.Unreadable)
	}
	if !holds(set.Unreadable, bench.WorkbenchKey) {
		t.Errorf("the workbench journal is not named in unreadable: %v", set.Unreadable)
	}
	if !holds(cardIDs(set), damagedID) {
		t.Errorf("the damaged card is not resynced into cards: %v", cardIDs(set))
	}
	for _, ev := range set.Events {
		if ev.ID == damagedID || ev.Scope == ScopeWorkbench {
			t.Errorf("an event was delivered out of a journal that would not parse: %+v", ev)
		}
	}
	if !holds(eventNames(set), contract.EventCommented) {
		t.Errorf("the sound card's own line was lost with the damaged one: %v", eventNames(set))
	}

	after := h.checkpoint(&Request{Since: set.Cursor})
	if after.Changed {
		t.Error("the cursor did not advance past the corruption, so it is reported forever")
	}

	// And once the board moves again, the sound card's earlier line stays
	// delivered rather than coming back with the new one.
	h.advance(time.Minute)
	h.reopen()
	h.comment(sound, "a later line on the sound card")
	later := h.checkpoint(&Request{Since: after.Cursor})
	if names := eventNames(later); len(names) != 1 {
		t.Errorf("the cursor did not advance past what it delivered: %v", names)
	}
}

// TestADeletedCardAndWorkstreamAreBothIdentifiersAndNeitherIsACard covers
// dinah-120 AC-14, and it fails on any implementation that reads a workbench
// deleted line as a gone card.
//
// The kind is asserted as the empty string on both entries rather than as
// absent-or-empty, so a later implementation that starts filling it has to
// change this test deliberately.
func TestADeletedCardAndWorkstreamAreBothIdentifiersAndNeitherIsACard(t *testing.T) {
	h := newHarness(t)
	card := h.add("A card somebody destroys")
	cardID := h.cardID(card)
	cardTitle := h.card(card).Title
	streamID, streamRef := h.workstream("A workstream somebody destroys")
	// A card that survives the window and is acted on inside it, so the two
	// destroyed identifiers are read against a cards array that has something
	// in it. On a board emptied by the deletions, "neither is reported as a
	// card" is a claim about an empty list and cannot fail.
	bystander := h.add("A card that survives")
	minted := h.mint()

	h.remove(card)
	h.remove(streamRef)
	h.comment(bystander, "a line on the card that stays")

	set := h.checkpoint(&Request{Since: minted})
	found := map[string]GoneEntity{}
	for _, entry := range set.Gone {
		found[entry.ID] = entry
	}
	if len(found) != 2 {
		t.Fatalf("wanted both departures, got %+v", set.Gone)
	}
	for id, wantedTitle := range map[string]string{cardID: cardTitle, streamID: "A workstream somebody destroys"} {
		entry, ok := found[id]
		if !ok {
			t.Errorf("%s is not in gone: %+v", id, set.Gone)
			continue
		}
		if entry.Fate != FateRemoved {
			t.Errorf("%s: wanted fate %s, got %q", id, FateRemoved, entry.Fate)
		}
		if entry.Kind != "" {
			t.Errorf("%s: a deleted event names no entity kind, and the entry claims %q", id, entry.Kind)
		}
		if entry.Title != wantedTitle {
			t.Errorf("%s: wanted the title as of deletion %q, got %q", id, wantedTitle, entry.Title)
		}
	}
	if got := cardIDs(set); len(got) != 1 || got[0] != h.cardID(bystander) {
		t.Errorf("wanted the surviving card alone in cards, got %v", got)
	}
	if holds(cardIDs(set), cardID) || holds(cardIDs(set), streamID) {
		t.Errorf("a destroyed identifier was reported as a live card: %v", cardIDs(set))
	}
}

// TestTheArchiveIsSkippedUntilTheArchiveItselfMoves covers dinah-120 AC-15,
// which pins D-9's gate through its one observable consequence.
//
// The discriminating assertion is the last one: an implementation that parses
// the archive on every call reports the corrupted journal again on the call
// after the reporting one, and this requires that call to be silent.
func TestTheArchiveIsSkippedUntilTheArchiveItselfMoves(t *testing.T) {
	h := newHarness(t)
	archived := h.add("A card in the archive")
	archivedID := h.cardID(archived)
	h.archive(archived)
	live := h.add("A card still on the board")
	minted := h.mint()

	h.mustDo(&Request{Verb: Move, Actor: "alka", Card: live, State: doing})
	confined := h.checkpoint(&Request{Since: minted})
	if len(confined.Gone) != 0 || len(confined.Unreadable) != 0 {
		t.Errorf("a change confined to the live half opened the archive: %+v %v", confined.Gone, confined.Unreadable)
	}

	mirror := filepath.Join(h.library.Bench.ArchivedCardsRoot(), archivedID, bench.JournalName)
	h.appendRaw(mirror, "this line is not JSON\n")
	if err := bench.AppendEvent(mirror, bench.Event{TS: bench.Stamp(h.clock.Add(time.Hour)), Event: contract.EventCommented, Actor: "alka"}); err != nil {
		t.Fatalf("append past the archived bad line: %v", err)
	}

	reported := h.checkpoint(&Request{Since: confined.Cursor})
	if !holds(reported.Unreadable, bench.CardsDir+"/"+archivedID) {
		t.Errorf("the corrupted archived journal is not reported: %v", reported.Unreadable)
	}

	// The live half moves and the archive does not, which is the call that
	// tells the two implementations apart: one that opens the archive on
	// every changed call reports the same corruption again here, and a gated
	// one says nothing about a half it did not read.
	h.advance(time.Minute)
	h.reopen()
	h.comment(live, "a line that moves the live term alone")

	after := h.checkpoint(&Request{Since: reported.Cursor})
	if !after.Changed {
		t.Fatal("the live act left the board reading unchanged, so this proves nothing")
	}
	if len(after.Unreadable) != 0 {
		t.Errorf("the archive was parsed again on a call whose archive term had not moved: %v", after.Unreadable)
	}
}

// TestAnInterruptedArchiveIsReportedGoneAndNotAlsoLive records the answer to
// dinah-120 OQ-11.
//
// StructuralAct writes the archived event before it moves the directory, so a
// crash between the two leaves the event in a journal that is still in the
// live half, with the anchor still beside it. The anchor is loaded live half
// first and mirror second, which is the order linkRef already reads in, and
// the card is suppressed from cards: an archived card has no live state a
// caller would act on, and one departure is reported in one place.
func TestAnInterruptedArchiveIsReportedGoneAndNotAlsoLive(t *testing.T) {
	h := newHarness(t)
	ref := h.add("A card whose archiving was interrupted")
	id := h.cardID(ref)
	wantedRef := h.card(ref).Ref(h.library.Bench.Slug)
	wantedTitle := h.card(ref).Title
	minted := h.mint()

	// The event without the move, which is exactly the window a crash leaves.
	if err := bench.AppendEvent(h.journalOf(ref), bench.Event{
		TS: bench.Stamp(h.clock.Add(time.Hour)), Event: contract.EventArchived, Actor: "alka", Note: id,
	}); err != nil {
		t.Fatalf("append the archived event: %v", err)
	}

	set := h.checkpoint(&Request{Since: minted})
	if len(set.Gone) != 1 || set.Gone[0].ID != id {
		t.Fatalf("wanted the interrupted archive in gone, got %+v", set.Gone)
	}
	if set.Gone[0].Kind != ScopeCard || set.Gone[0].Fate != FateArchived {
		t.Errorf("wanted kind card and fate archived, got %+v", set.Gone[0])
	}
	if set.Gone[0].Ref != wantedRef || set.Gone[0].Title != wantedTitle {
		t.Errorf("the entry was not named from the anchor still in the live half: %+v", set.Gone[0])
	}
	if holds(cardIDs(set), id) {
		t.Errorf("the departing card was reported as live as well as gone: %v", cardIDs(set))
	}
}

// TestAnArchivedWorkstreamLeavesTheWalkWithoutAnEntry records the answer to
// dinah-120 OQ-10, which the operator ruled on: the fact that something was
// archived is recorded, and the acts taken inside an archived entity are not.
//
// The archived half of the workstreams collection is outside the watched set,
// so archiving one moves the live digest term and names nothing in the answer.
// The caller learns the board moved, which is what changed is for.
//
// The board carries a card nothing in the window touches, and the last
// assertion reads what cards holds rather than only what it does not. On an
// empty board this test cannot see the answer it is recording, which is what
// cycle-3 review caught: the archived workstream's key leaves the live term
// and its own line is written into a journal the walk does not read, so the
// call has a moved live term and no delivered evidence at all and falls back
// to reporting every live card. That is a safe superset and it is D-13's
// stated bound, and it is a case where the evidence does exist on disk and is
// deliberately not read, which is worth stating out loud rather than leaving
// for a reader to infer from two other files.
func TestAnArchivedWorkstreamLeavesTheWalkWithoutAnEntry(t *testing.T) {
	h := newHarness(t)
	id, ref := h.workstream("A workstream on its way out")
	bystander := h.add("A card nothing in the window touches")
	minted := h.mint()

	h.archive(ref)

	set := h.checkpoint(&Request{Since: minted})
	if !set.Changed {
		t.Error("archiving a workstream left the board reading unchanged")
	}
	for _, entry := range set.Gone {
		if entry.ID == id {
			t.Errorf("the archived workstream is reported as gone, and the walk cannot prove what it was: %+v", entry)
		}
	}
	for _, ev := range set.Events {
		if ev.ID == id {
			t.Errorf("an act inside the archived workstream was delivered: %+v", ev)
		}
	}
	if holds(set.Unreadable, bench.WorkstreamsDir+"/"+id) {
		t.Errorf("the archived workstream is named in unreadable: %v", set.Unreadable)
	}
	// The whole-board resync this leaves behind, stated rather than implied.
	// A rule that started reading the archived workstreams half, or that
	// stopped resyncing on a departure it could not attribute, would have to
	// change this line deliberately.
	if got := cardIDs(set); len(got) != 1 || got[0] != h.cardID(bystander) {
		t.Errorf("wanted the untouched card resynced and nothing else, got %v", got)
	}
}

// TestAFilterRefusesOverTheNameItCannotResolve covers the two narrowing
// arguments' own refusals, which the check list declares and which a caller
// meets before any walk happens.
func TestAFilterRefusesOverTheNameItCannotResolve(t *testing.T) {
	h := newHarness(t)
	h.add("A card")
	minted := h.mint()

	cases := []struct {
		name    string
		request *Request
		refusal string
	}{
		{name: "an unknown card", request: &Request{Since: minted, Card: "fx-404"}, refusal: contract.UnknownCard},
		{name: "an unknown state", request: &Request{Since: minted, State: "nowhere"}, refusal: contract.UnknownState},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.request.Verb = "changes"
			if _, err := h.library.Changes(c.request); err == nil {
				t.Fatal("wanted a refusal, got an answer")
			} else if refusal, ok := err.(*contract.Refusal); !ok || refusal.Name != c.refusal {
				t.Errorf("wanted %s, got %v", c.refusal, err)
			}
		})
	}
}

// TestTheCursorIsBoundedByOneSecondRatherThanByTheBoard covers dinah-120
// AC-17, which is the property the token's whole shape was chosen for, stated
// as what the shape actually delivers.
//
// The cursor carries a boundary second and a frontier within that second, and
// the frontier holds one entry per entity with a covered line at the
// boundary. It is therefore bounded by how much happened inside one second
// and not by how much is on the board, which is what let gone be derived from
// events rather than from a list of keys the cursor carries. The second half
// below is the assertion that says so: a board of thirteen cards whose newest
// second holds two acts mints a token naming those two entities and no other.
func TestTheCursorIsBoundedByOneSecondRatherThanByTheBoard(t *testing.T) {
	h := newHarness(t)
	first := h.add("The first card")
	small := len(h.mint())
	var later []string
	for i := 0; i < 12; i++ {
		// A second apart, which is what a board being filled looks like.
		// Thirteen cards inside one frozen second would be one burst rather
		// than a board, and the frontier is sized by the burst on purpose.
		h.advance(time.Second)
		h.reopen()
		later = append(later, h.add("Card number "+strconv.Itoa(i)))
	}
	large := len(h.mint())
	if large > small+len(bench.CardsDir+"/")+16 {
		t.Errorf("the cursor grew from %d bytes to %d as the board did, and it is specified not to", small, large)
	}

	h.advance(time.Second)
	h.reopen()
	h.comment(first, "one act of the newest second")
	h.comment(later[0], "the other act of the newest second")
	held, err := decodeCursor(h.mint())
	if err != nil {
		t.Fatalf("decode the minted token: %v", err)
	}
	wanted := map[string]bool{
		bench.CardsDir + "/" + h.cardID(first):    true,
		bench.CardsDir + "/" + h.cardID(later[0]): true,
	}
	if len(held.Frontier) != len(wanted) {
		t.Fatalf("the newest second holds two acts on a board of thirteen cards, and the frontier carries %d entries: %v", len(held.Frontier), held.Frontier)
	}
	for key := range held.Frontier {
		if !wanted[key] {
			t.Errorf("the frontier names %s, which carries no line in the cursor's own second", key)
		}
	}
}

// TestAnActThatExplainsItselfDoesNotResyncTheWholeBoard is the test the
// cycle-1 code review asked for, and it is the one that tells the narrow
// fallback from the total one.
//
// dinah-120 D-13 bounds the whole-board resync at the case where the live term
// moved and the walk delivered nothing at all to explain it. Every other
// fixture that reaches the fallback has emptied the live half first, so an
// implementation counting card evidence alone and one counting every entity's
// evidence answer identically on all of them. Each case below leaves a card on
// the board that nothing in the window touched, so the two answers differ: the
// narrow rule leaves the bystander out and the total one hands it back.
func TestAnActThatExplainsItselfDoesNotResyncTheWholeBoard(t *testing.T) {
	t.Run("a workbench field rewrite", func(t *testing.T) {
		h := newHarness(t)
		first := h.add("A card nobody touches")
		second := h.add("Another card nobody touches")
		minted := h.mint()

		response := h.library.SetWorkbench(&Request{Verb: "workbench", Actor: "alka", Action: "set", Field: "title", Value: "A renamed workbench"})
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("workbench set title: %s %s", response.Outcome, response.Refusal)
		}
		h.reopen()

		set := h.checkpoint(&Request{Since: minted})
		if !set.Changed {
			t.Fatal("a workbench field rewrite left the board reading unchanged")
		}
		if len(set.Cards) != 0 {
			t.Errorf("a workbench act resynced cards that nothing touched: %v", cardIDs(set))
		}
		_, _ = first, second
	})

	t.Run("a workstream act", func(t *testing.T) {
		h := newHarness(t)
		h.add("A card nobody touches")
		h.add("Another card nobody touches")
		minted := h.mint()

		h.workstream("A workstream filed after the cursor")

		set := h.checkpoint(&Request{Since: minted})
		if !set.Changed {
			t.Fatal("a workstream act left the board reading unchanged")
		}
		if len(set.Cards) != 0 {
			t.Errorf("a workstream act resynced cards that nothing touched: %v", cardIDs(set))
		}
	})

	t.Run("a deleted card", func(t *testing.T) {
		h := newHarness(t)
		doomed := h.add("A card somebody destroys")
		bystander := h.add("A card nobody touches")
		minted := h.mint()

		h.remove(doomed)

		set := h.checkpoint(&Request{Since: minted})
		if len(set.Gone) != 1 {
			t.Fatalf("wanted the one departure, got %+v", set.Gone)
		}
		if holds(cardIDs(set), h.cardID(bystander)) {
			t.Errorf("a deletion resynced a card nothing touched: %v", cardIDs(set))
		}
	})

	t.Run("a completed archiving", func(t *testing.T) {
		h := newHarness(t)
		leaving := h.add("A card somebody archives")
		bystander := h.add("A card nobody touches")
		minted := h.mint()

		h.archive(leaving)

		set := h.checkpoint(&Request{Since: minted})
		if len(set.Gone) != 1 {
			t.Fatalf("wanted the one departure, got %+v", set.Gone)
		}
		if holds(cardIDs(set), h.cardID(bystander)) {
			t.Errorf("an archiving resynced a card nothing touched: %v", cardIDs(set))
		}
	})
}

// TestAFilterNamingADepartedCardStillAnswers covers dinah-120 AC-16, which is
// the criterion dinah-120 D-16 owes.
//
// The card an agent is parked on is the card most likely to leave, and the
// departure is what it was watching for, so a filter that refused the moment
// its subject left would close exactly when it was wanted. The refusal is kept
// for a reference that names nothing and is no identifier, which is what
// separates this ruling from dropping the check.
func TestAFilterNamingADepartedCardStillAnswers(t *testing.T) {
	t.Run("an archived card, named by the reference a person types", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("A card an agent is parked on")
		id := h.cardID(ref)
		h.add("A card the filter is not about")
		minted := h.mint()

		h.archive(ref)

		set := h.checkpoint(&Request{Since: minted, Card: ref})
		if len(set.Gone) != 1 || set.Gone[0].ID != id || set.Gone[0].Fate != FateArchived {
			t.Fatalf("the filter did not report the departure it was pointed at: %+v", set.Gone)
		}
		if len(set.Cards) != 0 {
			t.Errorf("an archived card was reported live to the filter naming it: %v", cardIDs(set))
		}
	})

	t.Run("a deleted card, named by the identifier the caller holds", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("A card an agent is parked on")
		id := h.cardID(ref)
		h.add("A card the filter is not about")
		minted := h.mint()

		h.remove(ref)

		set := h.checkpoint(&Request{Since: minted, Card: id})
		if len(set.Gone) != 1 || set.Gone[0].ID != id || set.Gone[0].Fate != FateRemoved {
			t.Fatalf("the filter did not report the destruction it was pointed at: %+v", set.Gone)
		}
	})

	t.Run("a reference that names nothing and is no identifier still refuses", func(t *testing.T) {
		h := newHarness(t)
		h.add("A card")
		minted := h.mint()

		_, err := h.library.Changes(&Request{Verb: "changes", Since: minted, Card: "fx-404"})
		if refusal, ok := err.(*contract.Refusal); !ok || refusal.Name != contract.UnknownCard {
			t.Errorf("wanted %s, got %v", contract.UnknownCard, err)
		}
	})
}

// sameSecondPair files two cards and returns their references with the one
// whose entity key sorts lower first, which is what lets the fixture below
// choose its subject rather than leave it to a random identifier.
func (h *harness) sameSecondPair() (lower, higher string) {
	h.t.Helper()
	first, second := h.add("The first card of the shared second"), h.add("The second card of the shared second")
	if h.cardID(first) < h.cardID(second) {
		return first, second
	}
	return second, first
}

// TestAnActInTheCursorsOwnSecondIsStillDelivered covers dinah-120 AC-20, which
// is the case the merged order cannot decide and the cursor therefore must
// not decide with it.
//
// bench.TimeFormat is second resolution, so two acts inside one second carry
// one stamp and the merged order separates them by entity key. That order is
// fine to present lines in and it is not the order they arrived in, so a
// cursor recorded as a single point in it classified the later of two acts as
// ancient history whenever that act's entity key sorted lower. The line was
// never delivered and never could be: a call that delivers nothing leaves the
// position where it was while the digest terms move on, so the call after it
// reports no change at all, and an archiving lost that way takes the whole
// departure with it.
//
// Every subtest below puts the cursor inside the second, which is why the
// harness no longer steps its clock after minting.
func TestAnActInTheCursorsOwnSecondIsStillDelivered(t *testing.T) {
	t.Run("a card whose key sorts below the cursor's own line", func(t *testing.T) {
		h := newHarness(t)
		lower, higher := h.sameSecondPair()
		// The act on the higher-sorting card puts the newest line of the
		// second on a key above the subject's, so the comparison this test is
		// about is the one the cursor actually faces.
		h.comment(higher, "the act the cursor is taken after")
		minted := h.mint()

		h.comment(lower, "the act the cursor was taken before")

		set := h.checkpoint(&Request{Since: minted})
		if len(set.Events) != 1 {
			t.Fatalf("wanted the act inside the cursor's own second, got %d: %v", len(set.Events), eventNames(set))
		}
		if got := set.Events[0].ID; got != h.cardID(lower) {
			t.Errorf("wanted the act on %s, got the one on %s", h.cardID(lower), got)
		}
		if !holds(cardIDs(set), h.cardID(lower)) {
			t.Errorf("the card acted on inside the cursor's second is not in cards: %v", cardIDs(set))
		}
		// Delivered once rather than on every call from here on.
		if again := h.checkpoint(&Request{Since: set.Cursor}); again.Changed || len(again.Events) != 0 {
			t.Errorf("the act was delivered a second time: changed=%v %v", again.Changed, eventNames(again))
		}
	})

	t.Run("an archiving in the cursor's own second", func(t *testing.T) {
		h := newHarness(t)
		leaving := h.add("A card the operator archives")
		leavingID := h.cardID(leaving)
		h.add("A card that stays")
		// A workstream key sorts above every card key, so the newest line of
		// the second is above the departure this fixture is about whatever
		// identifier the card happened to draw.
		h.workstream("A workstream acted on just before the cursor")
		minted := h.mint()

		h.archive(leaving)

		set := h.checkpoint(&Request{Since: minted})
		found := false
		for _, entry := range set.Gone {
			if entry.ID == leavingID {
				found = true
			}
		}
		if !found {
			t.Fatalf("the archiving inside the cursor's own second is reported nowhere: gone=%+v events=%v", set.Gone, eventNames(set))
		}
		if holds(cardIDs(set), leavingID) {
			t.Errorf("the archived card is reported live as well as gone: %v", cardIDs(set))
		}
		// And the loss it used to cause was permanent, so the call after the
		// reporting one has to be the silent one rather than the first one.
		if again := h.checkpoint(&Request{Since: set.Cursor}); again.Changed {
			t.Errorf("the cursor did not cover the departure it just reported: %+v", again.Gone)
		}
	})

	t.Run("a second act on the entity the cursor sits on", func(t *testing.T) {
		h := newHarness(t)
		card := h.add("A card acted on twice inside one second")
		h.comment(card, "the act the cursor is taken after")
		minted := h.mint()

		h.comment(card, "the act the cursor was taken before")

		set := h.checkpoint(&Request{Since: minted})
		if len(set.Events) != 1 || set.Events[0].ID != h.cardID(card) {
			t.Fatalf("wanted the later act on the same entity, got %v", eventNames(set))
		}
	})
}

// TestAFilteredCallIsToldTheDeletionJournalWentUnread covers dinah-120 AC-18.
//
// Deleted events are written to the workbench journal, so an unparseable one
// means gone cannot report a removal in that window. gone already exempts a
// removed entry from both filters, so a filtered caller does learn of its own
// card's destruction; the signal that the destruction could not be read has
// to be exempt too, or the caller is handed an empty answer with nothing in
// it to say that the news went unread, which is the silent absence of a
// departure this verb exists to prevent.
func TestAFilteredCallIsToldTheDeletionJournalWentUnread(t *testing.T) {
	h := newHarness(t)
	watched := h.add("The card the caller is watching")
	watchedID := h.cardID(watched)
	h.add("A card the caller is not watching")
	minted := h.mint()

	h.appendRaw(h.library.Bench.JournalPath(), "this line is not JSON\n")
	h.remove(watched)

	// The filter names the identifier rather than the reference, which is all
	// a caller holds once the card it was watching has been destroyed.
	set := h.checkpoint(&Request{Since: minted, Card: watchedID})
	if !set.Changed {
		t.Error("a card was destroyed and the filtered call reported the board unchanged")
	}
	if !holds(set.Unreadable, bench.WorkbenchKey) {
		t.Fatalf("the filtered caller is not told the deletion journal went unread: %v", set.Unreadable)
	}
	if len(set.Gone) != 0 {
		t.Errorf("the deleted event was read out of a journal that would not parse: %+v", set.Gone)
	}
}

// TestACorruptedArchiveDoesNotExplainAMovedLiveTerm covers dinah-120 AC-19.
//
// Evidence is counted before the board is resynced, and the two halves are
// not interchangeable evidence. A delivered archive line is a complete
// explanation of a moved live term, since an archiving is why the live key
// left. An archived journal that will not parse is not: it moves the archive
// term, it says nothing whatever about the live half, and letting it stand as
// the explanation suppresses a resync the live half had earned. The window is
// narrow and the card it loses is one D-13 already admits can be missed,
// which is why this is precision rather than a hole, and it is also why
// nothing else in the suite would have noticed.
func TestACorruptedArchiveDoesNotExplainAMovedLiveTerm(t *testing.T) {
	h := newHarness(t)
	edited := h.add("A card whose anchor is edited out of band")
	leaving := h.add("A card archived before the cursor")
	leavingID := h.cardID(leaving)
	h.archive(leaving)
	minted := h.mint()

	// The archived journal takes a bad line with a good one after it, which
	// is the shape ReadJournal refuses; a bad final line alone is a torn tail
	// and is tolerated.
	archived := filepath.Join(h.archivedDir(leavingID), bench.JournalName)
	h.appendRaw(archived, "this line is not JSON\n")
	if err := bench.AppendEvent(archived, bench.Event{TS: bench.Stamp(h.clock.Add(time.Hour)), Event: contract.EventCommented, Actor: "alka"}); err != nil {
		t.Fatalf("append past the bad line: %v", err)
	}

	// And the live half moves with no journal line anywhere, which is the one
	// case the walk has no evidence for.
	anchor := filepath.Join(h.library.Bench.CardsRoot(), h.cardID(edited), bench.CardAnchor)
	text, err := bench.ReadText(anchor)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	if err := bench.WriteText(anchor, text+"\nA paragraph somebody typed into the file.\n"); err != nil {
		t.Fatalf("rewrite the anchor: %v", err)
	}

	set := h.checkpoint(&Request{Since: minted})
	if !holds(set.Unreadable, bench.CardsDir+"/"+leavingID) {
		t.Errorf("the corrupted archived journal is not named in unreadable: %v", set.Unreadable)
	}
	if !holds(cardIDs(set), h.cardID(edited)) {
		t.Errorf("a corrupted archived journal was read as the explanation for a moved live term, and the edited card was lost: %v", cardIDs(set))
	}
}
