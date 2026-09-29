package bench

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"dinah/internal/contract"
)

// TestARedactionThatFailsBeforeItsRenameLeavesTheJournal drives the failure
// clause of dinah-637/criteria/38 on the migrated fixture. A redaction whose
// composed journal the filesystem refuses to rename over the journal answers
// that failure, leaves the journal byte for byte as it was, and leaves no
// journal.ndjson.redact behind. The failure is planted through redactStep, the
// package's own hook, as the storage migration's tests plant theirs.
//
// Arming: dropping the removal of the composed file on a failed rename leaves
// journal.ndjson.redact behind, which the last assertion catches.
func TestARedactionThatFailsBeforeItsRenameLeavesTheJournal(t *testing.T) {
	EnableCardUnitForTest(t)
	store := copyMigrationFixture(t)
	if report, err := migrateFixture(t, store, migrationRun(filepath.Join(t.TempDir(), "backup"))); err != nil || report.Outcome != contract.ReadOK {
		t.Fatalf("migrate: %v %+v", err, report)
	}
	opened, err := Open(store)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	card, err := opened.LoadCardIn(opened.CardsRoot(), mainCard)
	if err != nil {
		t.Fatalf("load %s: %v", mainCard, err)
	}
	record, err := opened.LoadCardRecord(card)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	var comment *Comment
	for _, held := range record.CommentsOf("", LiveHalf) {
		if held.Body != "" {
			comment = held
			break
		}
	}
	if comment == nil {
		t.Fatal("the fixture's main card carries no comment with a body")
	}
	target, found, err := opened.FindRedactTarget(card, "", KindComment, comment.ID)
	if err != nil || !found {
		t.Fatalf("find %s: %v %v", comment.ID, found, err)
	}
	journal := card.JournalPath()
	before, err := os.ReadFile(journal)
	if err != nil {
		t.Fatalf("read the journal: %v", err)
	}

	planted := errors.New("the disk refused the rename")
	plant(t, &redactStep, func(string) error { return planted })
	lock, err := opened.takeLock(card.Dir, "sam", "2026-09-28T13:00:00Z")
	if err != nil {
		t.Fatalf("lock: %v", err)
	}
	_, err = opened.Redact(lock, *target, Event{TS: "2026-09-28T13:00:00Z", Actor: NamedActor("sam")}, true)
	lock.Release()
	if !errors.Is(err, planted) {
		t.Errorf("the redaction answered %v, wanted the planted failure", err)
	}
	after, err := os.ReadFile(journal)
	if err != nil {
		t.Fatalf("read the journal again: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Error("the failed redaction changed the journal")
	}
	if Exists(journal + RedactLeftoverSuffix) {
		t.Error("the failed redaction left journal.ndjson.redact behind")
	}
}
