package bench

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"dinah/internal/contract"
	"dinah/internal/durable"
)

// LockRecord is the single JSON line a lock file carries, so that any process
// finding the lock can name its holder, including a careful human with an
// editor.
//
// Op and To belong to a sibling lock alone and are omitted from every other.
// OSLock belongs to an entity lock alone, and a sibling never carries it.
type LockRecord struct {
	// Actor is the owner the locking process was acting as.
	Actor string `json:"actor"`
	// PID is the locking process's identifier on the machine that took it.
	PID int `json:"pid"`
	// TS is when the lock was taken.
	TS string `json:"ts"`
	// Op is the structural operation the sibling stands for, one of
	// OpArchive, OpRestore and OpDelete.
	Op string `json:"op,omitempty"`
	// To is the path the directory is going to, empty for a removal.
	To string `json:"to,omitempty"`
	// Host is os.Hostname() of the process that took the lock.
	Host string `json:"host,omitempty"`
	// Start identifies that process and the process table it lives in, in
	// the form <runtime.GOOS>:<identity>, where the identity may be empty.
	Start string `json:"start,omitempty"`
	// OSLock is true when the holder took the operating-system lock on the
	// lock file before writing this record, and keeps it for as long as it
	// holds the lock.
	OSLock bool `json:"os_lock,omitempty"`
}

// LockName is the file that protects an entity directory. It sits directly
// inside the directory it protects, never inside the entity's frontmatter,
// because acquiring a lock by read-modify-write is a race and an in-band lock
// would churn the revision hash on every acquisition.
const LockName = "lock"

// SiblingSuffix is what a sibling lock's name adds to the identifier of the
// entity it stands beside, so `cards/<id>.lock` sits in the same collection
// as `cards/<id>/` and no reader that walks a collection can see it.
const SiblingSuffix = ".lock"

// The three structural operations a sibling lock records. An act that moves
// or removes an entity directory is one of these and nothing else.
const (
	OpArchive = "archive"
	OpRestore = "restore"
	OpDelete  = "delete"
)

// Lock is a held lock, released by calling Release.
//
// An entity lock keeps its lock file open, with the exclusive
// operating-system lock taken on it, for as long as it is held. The operating
// system releases that lock when the process ends, which is what lets another
// process prove the holder dead. A sibling lock and an adopted lock hold no
// handle.
type Lock struct {
	path   string
	actor  string
	file   *os.File
	hold   *durable.Hold
	record LockRecord
}

// self is this process's own host name and Start, computed once.
var self struct {
	once  sync.Once
	host  string
	start string
}

// selfIdentity answers this process's host name and its Start, the platform
// tag runtime.GOOS, a colon, and the identity processIdentity answers.
func selfIdentity() (string, string) {
	self.once.Do(func() {
		host, err := os.Hostname()
		if err != nil {
			host = ""
		}
		self.host = host
		self.start = runtime.GOOS + ":" + processIdentity()
	})
	return self.host, self.start
}

// newRecord composes the record this process writes into a lock it takes.
func newRecord(actor, now string) LockRecord {
	host, start := selfIdentity()
	return LockRecord{Actor: actor, PID: os.Getpid(), TS: now, Host: host, Start: start}
}

// SiblingPath names the lock that stands beside an entity directory for the
// length of a structural act on it: the directory's own name with the sibling
// suffix, in the same collection.
//
// The bench root has no sibling and gets an empty answer, so the check inside
// an acquisition skips it. The computed path would sit outside the bench tree
// entirely, no legitimate writer ever creates it, and a stray foreign file
// there would refuse every bench-scoped acquisition on the bench forever.
func SiblingPath(dir string) string {
	return siblingPath(Disk{}, dir)
}

// SiblingPath is the free SiblingPath read through this bench's source.
func (b *Bench) SiblingPath(dir string) string {
	return siblingPath(b.source(), dir)
}

// siblingPath is SiblingPath's body, reading through src.
func siblingPath(src Source, dir string) string {
	if exists(src, filepath.Join(dir, WorkbenchAnchor)) {
		return ""
	}
	return filepath.Join(filepath.Dir(dir), filepath.Base(dir)+SiblingSuffix)
}

// Acquire takes the lock of an entity directory. It records a reclaim of a
// dead holder's lock in the directory's own journal, and for a column's
// directory, which carries no journal, it never reclaims.
//
// The acquisition then reads for the sibling of a structural act on the same
// entity, and gives the lock straight back when it finds one. The check lives
// here rather than at each call site so that every acquirer inherits it and
// no future one can be written without it.
func Acquire(dir, actor string, now string) (*Lock, error) {
	return AcquireRecording(dir, NamedActor(actor), now, journalFor(dir))
}

// Acquire is the free Acquire reading the lock's sibling and holder through
// this bench's source.
func (b *Bench) Acquire(dir, actor string, now string) (*Lock, error) {
	return acquire(b.source(), dir, NamedActor(actor), now, journalFor(dir), nil)
}

// AcquireRecording takes the lock of an entity directory and names the
// journal a reclaim of a dead holder's lock is recorded in. The column
// occupancy lock is taken through it, recording in the journal of the card
// being moved or pulled, or the workbench journal for an add.
//
// A lock another process holds is refused with the holder named from the
// lock's own content. A lock whose holder is proven dead is reclaimed and the
// reclaim appended to journal as a lock_reclaimed line; anything short of
// that proof refuses. An acquisition with an empty journal, or whose actor
// names nobody, never reclaims.
func AcquireRecording(dir string, actor Actor, now string, journal string) (*Lock, error) {
	return acquire(Disk{}, dir, actor, now, journal, nil)
}

// AcquireRecording is the free AcquireRecording reading the lock's sibling
// and holder through this bench's source.
func (b *Bench) AcquireRecording(dir string, actor Actor, now string, journal string) (*Lock, error) {
	return acquire(b.source(), dir, actor, now, journal, nil)
}

// journalFor is the journal a lock on dir records a reclaim in: the
// directory's own journal, and none for a column's directory.
func journalFor(dir string) string {
	if filepath.Base(filepath.Dir(dir)) == ColumnsDir {
		return ""
	}
	return filepath.Join(dir, JournalName)
}

// columnOf answers the column identifier a lock directory belongs to, empty
// for any directory that is not a column's.
func columnOf(dir string) string {
	if filepath.Base(filepath.Dir(dir)) != ColumnsDir {
		return ""
	}
	return filepath.Base(dir)
}

// acquireTolerating takes an entity's lock for the one caller a sibling must
// not refuse: the structural act on its own subject, and the finish on the
// record it read back from the standing sibling. The tolerance matches the
// sibling's whole record rather than a flag, so a second process cannot ask
// for the same exemption and a stale sibling from a dead process grants none.
func acquireTolerating(src Source, dir, actor, now string, tolerated LockRecord) (*Lock, error) {
	return acquire(src, dir, NamedActor(actor), now, journalFor(dir), &tolerated)
}

// acquireAttempts bounds how many times an acquisition goes round when the
// lock file vanishes between its creation attempt and its verdict.
const acquireAttempts = 3

// acquire is the one exclusive-create of an entity lock file in this
// codebase, which is what keeps the sibling check unforgettable. It reads the
// holder's record and the sibling through src.
func acquire(src Source, dir string, actor Actor, now, journal string, tolerated *LockRecord) (*Lock, error) {
	path := filepath.Join(dir, LockName)
	for range acquireAttempts {
		lock := &Lock{path: path, actor: actor.Name}
		hold, owner := durable.Register(path, lock)
		if hold == nil {
			return nil, contract.Refuse(contract.Locked, holderNamed(src, owner, path))
		}
		lock.hold = hold
		f, err := durable.CreateExclusive(path)
		if err == nil {
			return lock.take(src, f, dir, now, tolerated)
		}
		if !errors.Is(err, fs.ErrExist) {
			hold.Unregister()
			return nil, err
		}
		if journal == "" || strings.TrimSpace(actor.Name) == "" {
			hold.Unregister()
			return nil, contract.Refuse(contract.Locked, lockHolder(src, path))
		}
		judged := judge(src, path)
		if judged.gone {
			hold.Unregister()
			continue
		}
		if judged.verdict != VerdictDead {
			hold.Unregister()
			return nil, contract.Refuse(contract.Locked, judged.record.Actor)
		}
		return lock.reclaim(src, judged, dir, actor, now, journal, tolerated)
	}
	return nil, contract.Refuse(contract.Locked, lockHolder(src, path))
}

// holderNamed names the holder of a lock this process already has an entry
// for: the actor of the acquisition that owns it, or, for a judge's entry,
// whatever the file itself records, read through src.
func holderNamed(src Source, owner any, path string) string {
	if held, ok := owner.(*Lock); ok && held.actor != "" {
		return held.actor
	}
	return lockHolder(src, path)
}

// take completes an acquisition whose exclusive create succeeded: it takes the
// operating-system lock, writes and flushes the record through the same
// handle, keeps the handle, and reads for the sibling.
//
// The operating-system lock is taken before the record is written, so any
// record a judge can parse was written by a process that held the lock when
// it wrote it. A file system offering no such lock is held without one, and
// the record says so.
func (l *Lock) take(src Source, f *os.File, dir, now string, tolerated *LockRecord) (*Lock, error) {
	taken, err := durable.TakeOSLock(f)
	if err != nil {
		f.Close()
		durable.RemoveLock(l.path)
		l.hold.Unregister()
		return nil, err
	}
	record := newRecord(l.actor, now)
	record.OSLock = taken
	if err := writeHeldRecord(f, record); err != nil {
		durable.DeleteHeld(f, l.path)
		durable.CloseLockFile(f)
		l.hold.Unregister()
		return nil, err
	}
	l.file = f
	l.record = record
	if err := l.refuseOnSibling(src, dir, tolerated); err != nil {
		return nil, err
	}
	return l, nil
}

// writeHeldRecord writes a record as the lock file's whole content through the
// holder's handle and flushes it.
func writeHeldRecord(f *os.File, record LockRecord) error {
	line, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return durable.WriteHeld(f, append(line, '\n'))
}

// AcquireSibling creates the lock that stands beside an entity directory for
// the length of a structural act, and returns the record it wrote so that the
// act can present it as the one sibling its own acquisitions tolerate. A
// sibling holds no handle and no operating-system lock, and is never
// reclaimed; dinah check --finish is its repair.
func AcquireSibling(dir, actor, now, op, to string) (*Lock, LockRecord, error) {
	return acquireSibling(Disk{}, dir, actor, now, op, to)
}

// AcquireSibling is the free AcquireSibling read through this bench's source.
func (b *Bench) AcquireSibling(dir, actor, now, op, to string) (*Lock, LockRecord, error) {
	return acquireSibling(b.source(), dir, actor, now, op, to)
}

// acquireSibling is AcquireSibling's body, reading through src.
func acquireSibling(src Source, dir, actor, now, op, to string) (*Lock, LockRecord, error) {
	record := newRecord(actor, now)
	record.Op = op
	record.To = to
	path := siblingPath(src, dir)
	if path == "" {
		return nil, record, contract.Refuse(contract.UnknownPath, dir)
	}
	f, err := durable.CreateExclusive(path)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return nil, record, contract.Refuse(contract.Locked, lockHolder(src, path))
		}
		return nil, record, err
	}
	if err := writeHeldRecord(f, record); err != nil {
		f.Close()
		durable.RemoveLock(path)
		return nil, record, err
	}
	if err := f.Close(); err != nil {
		durable.RemoveLock(path)
		return nil, record, err
	}
	return &Lock{path: path, actor: actor}, record, nil
}

// adoptLock names a lock file that already stands. The finish takes over the
// sibling an interrupted act left behind rather than creating one of its own,
// so step two is a read for a repair where it is a write for an act.
func adoptLock(path string) *Lock {
	return &Lock{path: path}
}

// refuseOnSibling gives a freshly taken lock back when a structural act's
// sibling stands beside the directory it protects, which is the window
// between the act's release of that lock and its move of the directory.
func (l *Lock) refuseOnSibling(src Source, dir string, tolerated *LockRecord) error {
	path := siblingPath(src, dir)
	if path == "" {
		return nil
	}
	record, present := readLockRecord(src, path)
	if !present {
		return nil
	}
	if tolerated != nil && record == *tolerated {
		return nil
	}
	l.Release()
	return contract.Refuse(contract.Locked, record.Actor)
}

// ReadLockRecord reads a lock file's own line. The second value reports
// whether a lock stands at that path at all, so a file whose content will not
// parse still counts as one held rather than as one absent.
func ReadLockRecord(path string) (LockRecord, bool) {
	return readLockRecord(Disk{}, path)
}

// ReadLockRecord is the free ReadLockRecord read through this bench's source.
func (b *Bench) ReadLockRecord(path string) (LockRecord, bool) {
	return readLockRecord(b.source(), path)
}

// readLockRecord is ReadLockRecord's body, reading through src.
func readLockRecord(src Source, path string) (LockRecord, bool) {
	text, err := readText(src, path)
	if err != nil {
		return LockRecord{}, false
	}
	record, _ := parseLockRecord(text)
	return record, true
}

// parseLockRecord parses the first line of a lock file's content, answering
// whether it parsed. An empty or unreadable record answers false.
func parseLockRecord(text string) (LockRecord, bool) {
	var record LockRecord
	lines := SplitLines(text)
	if len(lines) == 0 {
		return record, false
	}
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		return LockRecord{}, false
	}
	return record, true
}

// LockHolder names the owner recorded in a lock file, for the refusal that
// reports one. A lock whose content will not parse names no holder rather
// than failing, since the refusal is more useful than a second error.
func LockHolder(path string) string {
	return lockHolder(Disk{}, path)
}

// LockHolder is the free LockHolder read through this bench's source.
func (b *Bench) LockHolder(path string) string {
	return lockHolder(b.source(), path)
}

// lockHolder is LockHolder's body, reading through src.
func lockHolder(src Source, path string) string {
	record, _ := readLockRecord(src, path)
	return record.Actor
}

// Release lets the lock go. An entity lock's file is deleted through its own
// handle and the handle closed, which gives back the operating-system lock; a
// sibling's or an adopted lock's file is removed by name. A failure still
// closes the handle and removes the registry entry, and a file left behind is
// judged dead by the verdict's rule for this process's own orphan.
func (l *Lock) Release() {
	if l == nil || l.path == "" {
		return
	}
	if l.file != nil {
		durable.DeleteHeld(l.file, l.path)
		durable.CloseLockFile(l.file)
		l.file = nil
	} else {
		durable.RemoveLock(l.path)
	}
	l.hold.Unregister()
	l.hold = nil
	l.path = ""
}

// abandon lets the lock go the way a process that died lets it go: the handle
// is closed, which gives back the operating-system lock, and the registry
// entry goes with the process, but the file stays where it is. A test's
// ErrAborted stands for exactly that death.
func (l *Lock) abandon() {
	if l == nil || l.path == "" {
		return
	}
	if l.file != nil {
		durable.CloseLockFile(l.file)
		l.file = nil
	}
	l.hold.Unregister()
	l.hold = nil
	l.path = ""
}
