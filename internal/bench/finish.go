package bench

import (
	"os"
	"path/filepath"
	"strings"

	"dinah/internal/contract"
)

// interruption is one standing sibling: the record an interrupted structural
// act left behind, together with the two paths its directory can be at and
// the journal that decides which way it finishes.
type interruption struct {
	// path is the sibling lock file itself.
	path string
	// id is the identifier of the entity the act was about.
	id string
	// record is the sibling's own content, carrying the operation.
	record LockRecord
	// source is where the directory started, and target where the act was
	// taking it. A removal has no target.
	source string
	target string
	// journal is the journal whose event says whether the act reached its
	// point of record.
	journal string
	// lockDir is the directory whose lock the act took at its third step,
	// empty for an act whose scope is the bench.
	lockDir string
	// direction is what the finish will do, from the journal and the two
	// paths.
	direction string
}

// finding renders an interruption for the report. A directory at both paths
// is its own finding, because a rename is not atomic on every filesystem and
// choosing which half is complete would rest on the property this design
// refuses to assume.
func (in interruption) finding() Finding {
	if in.direction == directionBoth {
		return Finding{Path: in.path, Key: FindingEntityAtBothPaths, Detail: in.id}
	}
	detail := strings.Join([]string{in.id, in.record.Op, in.direction}, " ")
	return Finding{Path: in.path, Key: FindingInterruptedAct, Detail: detail}
}

// directionBoth is the fifth answer the direction can take, and it is not a
// direction the finish walks in, so it stays inside this file rather than
// joining the reported set.
const directionBoth = "both"

// interruptions reads every sibling standing on the bench. The walk covers
// the live half of every collection a structural act can name and never the
// archive mirrors, since a sibling always lives in the live half whichever
// way its entity is travelling, which keeps the cost bounded by the live set.
func (b *Bench) interruptions() []interruption {
	var standing []interruption
	for _, collection := range b.siblingCollections() {
		entries, err := os.ReadDir(collection)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, SiblingSuffix) {
				continue
			}
			id := strings.TrimSuffix(name, SiblingSuffix)
			if !IsID(id) {
				continue
			}
			path := filepath.Join(collection, name)
			record, present := ReadLockRecord(path)
			if !present {
				continue
			}
			standing = append(standing, b.readInterruption(collection, id, path, record))
		}
	}
	return standing
}

// readInterruption works out where one standing sibling's entity can be and
// which way the act it records finishes.
func (b *Bench) readInterruption(collection, id, path string, record LockRecord) interruption {
	live := filepath.Join(collection, id)
	archived := ArchiveTarget(live)
	in := interruption{path: path, id: id, record: record, source: live, target: archived}
	switch record.Op {
	case OpRestore:
		in.source, in.target = archived, live
	case OpDelete:
		in.target = ""
	}
	in.journal = b.decidingJournal(collection, in.source, record.Op)
	in.lockDir = b.entityLockDir(collection, in.source)
	in.direction = b.direction(in)
	return in
}

// entityLockDir names the directory whose lock the interrupted act took at
// its third step, which is the same nearest enclosing journal-bearing entity
// the act itself computed: the card's own directory for a card and for
// anything below one, and nothing at all for an act whose scope is the bench.
func (b *Bench) entityLockDir(collection, source string) string {
	owner := filepath.Dir(collection)
	if owner == b.Root {
		if filepath.Base(collection) == CardsDir {
			return source
		}
		return ""
	}
	if Exists(filepath.Join(owner, CardAnchor)) {
		return owner
	}
	return ""
}

// siblingCollections lists the live collection directories a structural act
// can leave a sibling in: every collection the workbench mounts, and every
// collection each live entity below it mounts in turn.
//
// The walk reads Contains rather than naming the collections here, so a kind
// gaining a collection is reachable by the interruption sweep without this
// function being edited.
func (b *Bench) siblingCollections() []string {
	var collections []string
	for _, mount := range Contains(KindWorkbench) {
		dir := filepath.Join(b.Root, mount.Dir)
		collections = append(collections, dir)
		for _, id := range ListIDs(dir) {
			collections = append(collections, b.collectionsBelow(filepath.Join(dir, id), mount.Kind)...)
		}
	}
	return collections
}

// collectionsBelow lists the live collection directories below one entity,
// walking the containment grammar down from the kind it names.
func (b *Bench) collectionsBelow(dir, kind string) []string {
	var collections []string
	for _, mount := range Contains(kind) {
		collection := filepath.Join(dir, mount.Dir)
		collections = append(collections, collection)
		for _, id := range ListIDs(collection) {
			collections = append(collections, b.collectionsBelow(filepath.Join(collection, id), mount.Kind)...)
		}
	}
	return collections
}

// decidingJournal names the journal whose event says whether an act reached
// its point of record, which is the nearest journal that survives the act. An
// entity's own journal travels with its directory, so an archive or a restore
// of a card is decided by the journal at the directory's source; a deletion
// destroys that journal, so it is decided by the one above.
func (b *Bench) decidingJournal(collection, source, op string) string {
	owner := filepath.Dir(collection)
	if owner == b.Root {
		if filepath.Base(collection) == CardsDir && op != OpDelete {
			return filepath.Join(source, JournalName)
		}
		return b.JournalPath()
	}
	if Exists(filepath.Join(owner, CardAnchor)) {
		return filepath.Join(owner, JournalName)
	}
	return b.JournalPath()
}

// direction reads the two paths and the journal and says which way the act
// finishes.
func (b *Bench) direction(in interruption) string {
	if in.target != "" {
		sourceThere, targetThere := Exists(in.source), Exists(in.target)
		if sourceThere && targetThere {
			return directionBoth
		}
		if !sourceThere && !targetThere {
			return DirectionMissing
		}
		if !sourceThere {
			return DirectionForward
		}
	} else if !Exists(in.source) {
		return DirectionForward
	}
	if b.recorded(in) {
		return DirectionForward
	}
	return DirectionRollback
}

// recorded reports whether the act's own event is already on the journal that
// decides its direction.
func (b *Bench) recorded(in interruption) bool {
	events, _, err := ReadJournal(in.journal)
	if err != nil {
		return false
	}
	for _, ev := range events {
		if eventRecords(ev, in.record.Op, in.id) {
			return true
		}
	}
	return false
}

// eventRecords reports whether one journal line is the point of record of an
// act on an entity. A deleted attachment keeps the attachment event it has
// always carried; everything else is named by the act's own event with the
// identifier in the note.
func eventRecords(ev Event, op, id string) bool {
	if op == OpDelete && ev.Event == contract.EventAttachmentRemoved {
		return ev.Attachment == id
	}
	if ev.Note != id {
		return false
	}
	switch op {
	case OpArchive:
		return ev.Event == contract.EventArchived
	case OpRestore:
		return ev.Event == contract.EventRestored
	case OpDelete:
		return ev.Event == contract.EventDeleted
	}
	return false
}

// FinishInterrupted completes or rolls back every structural act an
// interruption left standing, and returns what it would not resolve.
//
// The bench lock is the liveness test and no separate one is needed, since a
// live act holds that lock from its first acquisition to its last and the
// live-failure path releases it while leaving the sibling standing. So a
// sibling found while the lock is free is stale by construction, and a
// sibling found while it is held means an act is running and the finish is
// refused. A bench lock an interrupted process left behind refuses the finish
// until a human clears it, which is the stale-lock rule rather than an
// exception to it.
func (b *Bench) FinishInterrupted(actor, now string) ([]Finding, error) {
	benchLock, err := Acquire(b.Root, actor, now)
	if err != nil {
		return nil, err
	}
	defer benchLock.Release()
	var findings []Finding
	for _, standing := range b.interruptions() {
		finding, err := b.finish(standing)
		if err != nil {
			return findings, err
		}
		if finding != nil {
			findings = append(findings, *finding)
		}
	}
	return findings, nil
}

// finish walks one interrupted act to its end. It adopts the standing sibling
// rather than creating one, so the second step of the protocol is a read here
// where it is a write for the act itself.
func (b *Bench) finish(in interruption) (*Finding, error) {
	if in.direction == directionBoth || in.direction == DirectionMissing {
		finding := in.finding()
		return &finding, nil
	}
	entityLock, err := b.adoptEntityLock(in)
	if err != nil {
		return nil, err
	}
	if entityLock == nil {
		blocked := in.finding()
		blocked.Detail = strings.Join([]string{in.id, in.record.Op, DirectionLocked}, " ")
		return &blocked, nil
	}
	// A lock may no more travel into an archive from a repair than from a
	// live act, so it comes off before the directory moves.
	entityLock.Release()
	if in.direction == DirectionForward {
		if err := b.complete(in); err != nil {
			return nil, err
		}
	}
	adoptLock(in.path).Release()
	return nil, nil
}

// adoptEntityLock takes the entity's own lock through the tolerant acquire
// keyed to the record the standing sibling carries, since an ordinary acquire
// would be refused by the very sibling the finish exists to clear.
//
// A nil lock and a nil error report a lock the act did not take: one naming
// another actor or another process belongs to something live, so the finish
// stops rather than breaking it.
func (b *Bench) adoptEntityLock(in interruption) (*Lock, error) {
	if in.lockDir == "" {
		return adoptLock(""), nil
	}
	path := filepath.Join(in.lockDir, LockName)
	if record, present := ReadLockRecord(path); present {
		if record.Actor != in.record.Actor || record.PID != in.record.PID {
			return nil, nil
		}
		return adoptLock(path), nil
	}
	if !Exists(in.lockDir) {
		return adoptLock(path), nil
	}
	lock, err := acquireTolerating(in.lockDir, in.record.Actor, in.record.TS, in.record)
	if err != nil {
		return nil, err
	}
	return lock, nil
}

// complete carries out the move or the removal the interrupted act was past
// its point of record for. A directory already gone leaves nothing to do,
// which is what makes a second finish over the same bench change nothing.
func (b *Bench) complete(in interruption) error {
	if !Exists(in.source) {
		return nil
	}
	if in.target == "" {
		return DeleteEntity(in.source)
	}
	return MoveEntity(in.source, in.target)
}
