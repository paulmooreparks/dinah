package bench

import (
	"path/filepath"
	"time"
)

// FindingStaleLock names an entity lock whose holder check could not show to
// be live: one judged dead, which dinah check --finish clears, and one judged
// unknown, which only a person clears. Detail is the holder followed by the
// verdict, or the verdict alone for a record that names nobody. A live lock
// is work in flight and is never reported.
const FindingStaleLock = "check.stale-lock"

// The catalog keys of the next step a stale-lock finding carries, one per
// verdict it is reported under.
const (
	staleLockDeadNext    = "check.stale-lock.dead.next"
	staleLockUnknownNext = "check.stale-lock.unknown.next"
)

// ClearedLock is one dead lock dinah check --finish reclaimed and released.
type ClearedLock struct {
	// Path is the lock file that was cleared.
	Path string
	// Actor is the owner the dead record named.
	Actor string
	// PID is the process the dead record named.
	PID int
}

// entityLockFiles answers every entity lock file standing on the workbench:
// the workbench's own, and the one in each card, workstream and column
// directory of the live and the archived halves.
func (b *Bench) entityLockFiles() []string {
	var found []string
	root := filepath.Join(b.Root, LockName)
	if Exists(root) {
		found = append(found, root)
	}
	for _, half := range []string{b.Root, filepath.Join(b.Root, ArchiveDir)} {
		for _, collection := range []string{CardsDir, WorkstreamsDir, ColumnsDir} {
			found = append(found, lockFilesIn(filepath.Join(half, collection))...)
		}
	}
	return found
}

// lockFilesIn answers the lock file standing in each entity directory of one
// collection. A collection that cannot be listed holds none this walk can
// report.
func lockFilesIn(collection string) []string {
	ids, err := ListIDs(collection)
	if err != nil {
		return nil
	}
	var found []string
	for _, id := range ids {
		path := filepath.Join(collection, id, LockName)
		if Exists(path) {
			found = append(found, path)
		}
	}
	return found
}

// checkStaleLocks judges every entity lock file on the workbench and reports
// each one whose holder is dead or cannot be proven live.
func (b *Bench) checkStaleLocks() []Finding {
	var findings []Finding
	for _, path := range b.entityLockFiles() {
		record, verdict := JudgeLock(path)
		if verdict == VerdictLive {
			continue
		}
		findings = append(findings, staleLockFinding(path, record, verdict))
	}
	return findings
}

// staleLockFinding composes the finding for one lock judged dead or unknown.
func staleLockFinding(path string, record LockRecord, verdict Verdict) Finding {
	detail := verdict.String()
	if record.Actor != "" {
		detail = record.Actor + " " + detail
	}
	next := staleLockUnknownNext
	if verdict == VerdictDead {
		next = staleLockDeadNext
	}
	return Finding{Path: path, Key: FindingStaleLock, Detail: detail, Next: next}
}

// tornUnderLock answers whether a card journal whose lock-free read ended in a
// partial line still ends in one when it is read again under the card's lock.
// A reader holding no lock may see a final line an appender has only partly
// written, and every append to a card journal is made under that card's lock,
// so the second read is the one that can tell a tear from an append in
// flight. When another holder has the lock, an append may be in flight and
// nothing is reported this run.
func tornUnderLock(card *Card) bool {
	lock, err := Acquire(card.Dir, "", Stamp(time.Now()))
	if err != nil {
		return false
	}
	defer lock.Release()
	_, torn, err := ReadJournal(card.JournalPath())
	return err == nil && torn
}

// clearDeadLocks reclaims every entity lock judged dead, which records a
// lock_reclaimed line in the journal of the entity it protected, and then
// releases it, leaving the entity unlocked. The caller holds the workbench
// lock, so a column's lock records in the workbench journal. A lock inside a
// directory whose sibling stands belongs to the interrupted act's own finish
// and is left to it, and a lock judged unknown is left for a person.
func (b *Bench) clearDeadLocks(actor, now string) []ClearedLock {
	var cleared []ClearedLock
	rootLock := filepath.Join(b.Root, LockName)
	for _, path := range b.entityLockFiles() {
		if path == rootLock {
			continue
		}
		dir := filepath.Dir(path)
		if Exists(SiblingPath(dir)) {
			continue
		}
		record, verdict := JudgeLock(path)
		if verdict != VerdictDead {
			continue
		}
		journal := journalFor(dir)
		if journal == "" {
			journal = filepath.Join(b.Root, JournalName)
		}
		lock, err := AcquireRecording(dir, NamedActor(actor), now, journal)
		if err != nil {
			continue
		}
		lock.Release()
		cleared = append(cleared, ClearedLock{Path: path, Actor: record.Actor, PID: record.PID})
	}
	return cleared
}
