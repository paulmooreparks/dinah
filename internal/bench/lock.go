package bench

import (
	"encoding/json"
	"os"
	"path/filepath"

	"dinah/internal/contract"
)

// LockRecord is the single JSON line a lock file carries, so that any process
// finding the lock can name its holder, including a careful human with an
// editor.
//
// The last two members belong to a sibling lock alone and are omitted from
// every other, so an ordinary lock line is the same bytes it has always been
// and a hand-written one stays valid.
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

// Lock is a held entity lock, released by calling Release.
type Lock struct {
	path string
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
	if Exists(filepath.Join(dir, WorkbenchAnchor)) {
		return ""
	}
	return filepath.Join(filepath.Dir(dir), filepath.Base(dir)+SiblingSuffix)
}

// Acquire takes the lock of an entity directory with the filesystem's
// exclusive-create primitive, which is the whole of the mutual exclusion. A
// lock another process holds is refused loudly with the holder named from the
// lock's own content; nothing here ever breaks one silently.
//
// The acquisition then reads for the sibling of a structural act on the same
// entity, and gives the lock straight back when it finds one. The check lives
// here rather than at each call site so that every acquirer inherits it and
// no future one can be written without it.
func Acquire(dir, actor string, now string) (*Lock, error) {
	return acquire(dir, actor, now, nil)
}

// acquireTolerating takes an entity's lock for the one caller a sibling must
// not refuse: the structural act on its own subject, and the finish on the
// record it read back from the standing sibling. The tolerance matches the
// sibling's whole record rather than a flag, so a second process cannot ask
// for the same exemption and a stale sibling from a dead process grants none.
func acquireTolerating(dir, actor, now string, tolerated LockRecord) (*Lock, error) {
	return acquire(dir, actor, now, &tolerated)
}

// acquire is the one exclusive-create of a lock file in this codebase, which
// is what keeps the sibling check unforgettable.
func acquire(dir, actor, now string, tolerated *LockRecord) (*Lock, error) {
	path := filepath.Join(dir, LockName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, contract.Refuse(contract.Locked, LockHolder(path))
		}
		return nil, err
	}
	record := LockRecord{Actor: actor, PID: os.Getpid(), TS: now}
	if err := writeRecord(f, path, record); err != nil {
		return nil, err
	}
	lock := &Lock{path: path}
	if err := lock.refuseOnSibling(dir, tolerated); err != nil {
		return nil, err
	}
	return lock, nil
}

// AcquireSibling creates the lock that stands beside an entity directory for
// the length of a structural act, and returns the record it wrote so that the
// act can present it as the one sibling its own acquisitions tolerate.
func AcquireSibling(dir, actor, now, op, to string) (*Lock, LockRecord, error) {
	record := LockRecord{Actor: actor, PID: os.Getpid(), TS: now, Op: op, To: to}
	path := SiblingPath(dir)
	if path == "" {
		return nil, record, contract.Refuse(contract.UnknownPath, dir)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, record, contract.Refuse(contract.Locked, LockHolder(path))
		}
		return nil, record, err
	}
	if err := writeRecord(f, path, record); err != nil {
		return nil, record, err
	}
	return &Lock{path: path}, record, nil
}

// adoptLock names a lock file that already stands. The finish takes over the
// sibling an interrupted act left behind rather than creating one of its own,
// so step two is a read for a repair where it is a write for an act.
func adoptLock(path string) *Lock {
	return &Lock{path: path}
}

// writeRecord puts the lock's own line in the file the acquisition just
// created, removing the file again on any failure so a lock nobody holds is
// never left behind.
func writeRecord(f *os.File, path string, record LockRecord) error {
	line, err := json.Marshal(record)
	if err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return err
	}
	return nil
}

// refuseOnSibling gives a freshly taken lock back when a structural act's
// sibling stands beside the directory it protects, which is the window
// between the act's release of that lock and its move of the directory.
func (l *Lock) refuseOnSibling(dir string, tolerated *LockRecord) error {
	path := SiblingPath(dir)
	if path == "" {
		return nil
	}
	record, present := ReadLockRecord(path)
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
	text, err := ReadText(path)
	if err != nil {
		return LockRecord{}, false
	}
	var record LockRecord
	lines := SplitLines(text)
	if len(lines) == 0 {
		return record, true
	}
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		return LockRecord{}, true
	}
	return record, true
}

// LockHolder names the owner recorded in a lock file, for the refusal that
// reports one. A lock whose content will not parse names no holder rather
// than failing, since the refusal is more useful than a second error.
func LockHolder(path string) string {
	record, _ := ReadLockRecord(path)
	return record.Actor
}

// Release removes the lock, which is a plain deletion after the protected
// write's rename has landed.
func (l *Lock) Release() {
	if l == nil {
		return
	}
	os.Remove(l.path)
}
