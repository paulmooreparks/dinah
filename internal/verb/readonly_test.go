package verb

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// TestAReadOnlyLibraryWritesNoLapse is part of dinah-619/criteria/10. On a
// card claimed with an expiry, a ReadOnly library whose clock is past the
// expiry answers ErrReadOnly from Status and from Show of the card, and
// writes nothing: the anchor and the journal keep their bytes and no lock
// file appears. The same library without ReadOnly lapses the claim, as a
// read always has.
//
// Arming: removing the ReadOnly check from lapseRead lets Status journal the
// lapse, which reddens the error assertion and the unchanged-journal one.
func TestAReadOnlyLibraryWritesNoLapse(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("leased")
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "bob", Expires: 2 * time.Hour})
	card := h.card(ref)
	if card.Expires == "" {
		t.Fatal("the claim recorded no expiry")
	}
	anchor, journal, lock := card.AnchorPath(), card.JournalPath(), filepath.Join(card.Dir, bench.LockName)
	anchorBefore, journalBefore := readBytes(t, anchor), readBytes(t, journal)

	h.advance(3 * time.Hour)
	h.reopen()
	h.library.ReadOnly = true
	if _, err := h.library.Status(&Request{Verb: "status", Actor: "alka"}); !errors.Is(err, ErrReadOnly) {
		t.Errorf("Status on a read-only library past the expiry answered %v, wanted ErrReadOnly", err)
	}
	if _, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref}); !errors.Is(err, ErrReadOnly) {
		t.Errorf("Show of the card on a read-only library past the expiry answered %v, wanted ErrReadOnly", err)
	}
	if got := readBytes(t, anchor); got != anchorBefore {
		t.Error("the read-only reads changed the card's anchor")
	}
	if got := readBytes(t, journal); got != journalBefore {
		t.Error("the read-only reads changed the card's journal")
	}
	if _, err := os.Stat(lock); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the read-only reads left a lock file: %v", err)
	}

	h.library.ReadOnly = false
	if _, err := h.library.Status(&Request{Verb: "status", Actor: "alka"}); err != nil {
		t.Fatalf("Status without ReadOnly: %v", err)
	}
	lapsed := false
	for _, event := range h.events(ref) {
		if event.Event == contract.EventExpired {
			lapsed = true
		}
	}
	if !lapsed {
		t.Error("Status without ReadOnly did not lapse the claim")
	}
}

// readBytes reads a file's bytes as a string.
func readBytes(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
