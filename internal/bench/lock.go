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
type LockRecord struct {
	// Actor is the owner the locking process was acting as.
	Actor string `json:"actor"`
	// PID is the locking process's identifier on the machine that took it.
	PID int `json:"pid"`
	// TS is when the lock was taken.
	TS string `json:"ts"`
}

// LockName is the file that protects an entity directory. It sits directly
// inside the directory it protects, never inside the entity's frontmatter,
// because acquiring a lock by read-modify-write is a race and an in-band lock
// would churn the revision hash on every acquisition.
const LockName = "lock"

// Lock is a held entity lock, released by calling Release.
type Lock struct {
	path string
}

// Acquire takes the lock of an entity directory with the filesystem's
// exclusive-create primitive, which is the whole of the mutual exclusion. A
// lock another process holds is refused loudly with the holder named from the
// lock's own content; nothing here ever breaks one silently.
func Acquire(dir, actor string, now string) (*Lock, error) {
	path := filepath.Join(dir, LockName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err == nil {
		record := LockRecord{Actor: actor, PID: os.Getpid(), TS: now}
		line, marshalErr := json.Marshal(record)
		if marshalErr != nil {
			f.Close()
			os.Remove(path)
			return nil, marshalErr
		}
		if _, writeErr := f.Write(append(line, '\n')); writeErr != nil {
			f.Close()
			os.Remove(path)
			return nil, writeErr
		}
		if closeErr := f.Close(); closeErr != nil {
			os.Remove(path)
			return nil, closeErr
		}
		return &Lock{path: path}, nil
	}
	if !os.IsExist(err) {
		return nil, err
	}
	return nil, contract.Refuse(contract.Locked, LockHolder(path))
}

// LockHolder names the owner recorded in a lock file, for the refusal that
// reports one. A lock whose content will not parse names no holder rather
// than failing, since the refusal is more useful than a second error.
func LockHolder(path string) string {
	text, err := ReadText(path)
	if err != nil {
		return ""
	}
	var record LockRecord
	lines := SplitLines(text)
	if len(lines) == 0 {
		return ""
	}
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		return ""
	}
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
