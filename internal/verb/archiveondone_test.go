package verb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// TestArchiveOnDoneArchivesImmediatelyThroughLibraryDo is
// dinah-634/criteria/3: a Move landing in a done-kind column reports
// Archived true, the card is unreachable by Show and reachable through
// Bench.ResolveEntityIn(ArchivedHalf, ...), and the refusing pair, a move to
// a column that is not done-kind, carries no Archived and archives nothing.
func TestArchiveOnDoneArchivesImmediatelyThroughLibraryDo(t *testing.T) {
	h := newHarness(t)
	ref := h.add("finishing")
	response := h.library.Do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: finished})
	if !response.Archived {
		t.Fatalf("wanted Archived true, got %+v", response)
	}
	if _, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref}); err == nil {
		t.Fatal("wanted the live show to refuse, got none")
	} else if refusal, ok := err.(*contract.Refusal); !ok || refusal.Name != contract.UnknownCard {
		t.Fatalf("wanted unknown-card, got %v", err)
	}
	if _, err := h.library.Bench.ResolveEntityIn(bench.ArchivedHalf, ref); err != nil {
		t.Fatalf("wanted the archived half to resolve the card, got %v", err)
	}

	other := h.ready("staying live")
	plain := h.library.Do(&Request{Verb: Move, Card: other, Actor: "alka", Column: doing})
	if plain.Archived {
		t.Fatalf("a move to a non-done column set Archived: %+v", plain)
	}
	if _, err := h.library.Bench.ResolveEntityIn(bench.ArchivedHalf, other); err == nil {
		t.Fatal("a move to a non-done column archived the card")
	}
}

// TestArchivedCardsJournalRecordsMovedThenArchived is dinah-634/criteria/2.
func TestArchivedCardsJournalRecordsMovedThenArchived(t *testing.T) {
	h := newHarness(t)
	ref := h.add("finishing")
	id := h.card(ref).ID
	response := h.library.Do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: finished})
	if !response.Archived {
		t.Fatalf("wanted Archived true, got %+v", response)
	}
	journal := filepath.Join(h.library.Bench.ArchivedCardsRoot(), id, bench.JournalName)
	events, _, err := bench.ReadJournal(journal)
	if err != nil {
		t.Fatalf("read the archived journal: %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("wanted at least two events, got %d: %+v", len(events), events)
	}
	moved, archived := events[len(events)-2], events[len(events)-1]
	if moved.Event != contract.EventMoved || moved.To != finished {
		t.Errorf("the second-to-last event is %+v, wanted a moved event naming %s", moved, finished)
	}
	if moved.Actor.Name != "alka" {
		t.Errorf("the moved event names actor %q, wanted alka", moved.Actor.Name)
	}
	if archived.Event != contract.EventArchived || archived.Note != id {
		t.Errorf("the last event is %+v, wanted an archived event naming %s", archived, id)
	}
	if archived.Actor.Name != "alka" {
		t.Errorf("the archived event names actor %q, wanted alka", archived.Actor.Name)
	}
}

// TestNoArchiveLeavesTheCardLiveAndCheckReportsIt is dinah-634/criteria/5.
func TestNoArchiveLeavesTheCardLiveAndCheckReportsIt(t *testing.T) {
	h := newHarness(t)
	ref := h.add("held back")
	response := h.library.Do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: finished, NoArchive: true})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("the move: %s %s", response.Outcome, response.Refusal)
	}
	if response.Archived {
		t.Fatalf("--no-archive still archived: %+v", response)
	}
	if _, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref}); err != nil {
		t.Fatalf("wanted the card still live, got %v", err)
	}
	findings := h.check()
	found := false
	for _, finding := range findings {
		if finding.Key == bench.FindingUnarchivedDone {
			found = true
			if finding.Detail != "finished" {
				t.Errorf("the finding names %q, wanted the column's slug finished", finding.Detail)
			}
		}
	}
	if !found {
		t.Fatalf("check did not report %s, findings: %+v", bench.FindingUnarchivedDone, findings)
	}
}

// TestOrdinaryMoveIsBitForBitUnaffected is dinah-634/criteria/12.
func TestOrdinaryMoveIsBitForBitUnaffected(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("plain move")
	before := journalLength(t, h.card(ref).JournalPath())
	response := h.library.Do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: doing})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("the move: %s %s", response.Outcome, response.Refusal)
	}
	if response.Archived {
		t.Errorf("an ordinary move set Archived: %+v", response)
	}
	if response.Warning == "warn.archive-on-done-failed" {
		t.Errorf("an ordinary move carried the archive-failure warning: %+v", response)
	}
	after := journalLength(t, h.card(ref).JournalPath())
	if after != before+1 {
		t.Fatalf("wanted exactly one new journal line, got %d before and %d after", before, after)
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), `"archived"`) {
		t.Errorf("the response serialises an archived key: %s", encoded)
	}
}

// TestArchiveFailurePrecedesStalePrefixWarning is dinah-634/criteria/13.
func TestArchiveFailurePrecedesStalePrefixWarning(t *testing.T) {
	h := newHarness(t)
	ref := h.add("stale and blocked")
	id := h.card(ref).ID
	number := strings.TrimPrefix(ref, "fx-")
	// Pre-creating the archive target's directory makes Archive's own
	// structural act refuse rather than succeed, which is the engineered
	// failure section 3 of the specification calls for.
	target := filepath.Join(h.library.Bench.ArchivedCardsRoot(), id)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("plant the archive target: %v", err)
	}
	response := h.library.Do(&Request{Verb: Move, Card: "yokoten-" + number, Actor: "alka", Column: finished})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("the move itself: %s %s", response.Outcome, response.Refusal)
	}
	if response.Archived {
		t.Fatalf("the engineered failure still reported Archived true: %+v", response)
	}
	if response.Warning != "warn.archive-on-done-failed" {
		t.Fatalf("wanted warn.archive-on-done-failed to survive over the stale-prefix warning, got %q %q",
			response.Warning, response.WarningDetail)
	}
}

// TestAPendingItemStopsReportingOnceTheCardArchives is dinah-634/criteria/9.
func TestAPendingItemStopsReportingOnceTheCardArchives(t *testing.T) {
	h := newHarness(t)
	ref := h.readyAt("carries a question", doing)
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	filePrimeItem(h, ref, "open_question", "is this right", "holder", "")
	primer := mustPrime(h, "alka")
	found := false
	for _, item := range primer.Pending {
		if item.Card == ref {
			found = true
		}
	}
	if !found {
		t.Fatalf("the pending item is not reported while the card is live: %+v", primer.Pending)
	}
	// A done-kind column takes no held card, so the holder releases
	// immediately before the move that lands and archives it, on the terms
	// section 6 of the specification describes.
	h.mustDo(&Request{Verb: Release, Card: ref, Actor: "alka"})
	response := h.library.Do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: finished})
	if !response.Archived {
		t.Fatalf("wanted the card to archive, got %+v", response)
	}
	after := mustPrime(h, "alka")
	for _, item := range after.Pending {
		if item.Card == ref {
			t.Fatalf("the item is still reported after the card archived: %+v", after.Pending)
		}
	}
}

// TestRestoringAnArchiveOnDoneCardReturnsItLiveWithHistoryIntact is
// dinah-634/criteria/11.
func TestRestoringAnArchiveOnDoneCardReturnsItLiveWithHistoryIntact(t *testing.T) {
	h := newHarness(t)
	ref := h.add("carries history")
	h.comment(ref, "a note before finishing")
	h.item(ref, "e00000000001", "kind: acceptance_criterion\nstate: pending\n", "check the release")
	beforeEvents := len(h.events(ref))
	response := h.library.Do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: finished})
	if !response.Archived {
		t.Fatalf("wanted the card to archive, got %+v", response)
	}
	restored := h.library.Restore(&Request{Verb: "restore", Actor: "alka", Ref: ref})
	if restored.Outcome != contract.OutcomeOK {
		t.Fatalf("restore: %s %s", restored.Outcome, restored.Refusal)
	}
	h.reopen()
	card := h.card(ref)
	if card.Column != finished {
		t.Errorf("the restored card stands in %q, wanted %s", card.Column, finished)
	}
	afterEvents := h.events(ref)
	// Restoring is not itself a silent act: it appends its own line, so the
	// journal carries at least what it did before archiving plus that one.
	if len(afterEvents) < beforeEvents+1 {
		t.Errorf("the restored journal carries %d events, wanted at least %d", len(afterEvents), beforeEvents+1)
	}
	comments, err := bench.ListIDs(filepath.Join(card.Dir, bench.CommentsDir))
	if err != nil {
		t.Fatalf("list comments: %v", err)
	}
	if len(comments) != 1 {
		t.Errorf("the restored card carries %d comments, wanted 1", len(comments))
	}
	items, err := bench.ListIDs(filepath.Join(card.Dir, bench.ChecklistDir))
	if err != nil {
		t.Fatalf("list checklist items: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("the restored card carries %d checklist items, wanted 1", len(items))
	}
}
