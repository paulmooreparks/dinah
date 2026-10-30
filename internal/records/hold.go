package records

import (
	"errors"
	"os"
	"path/filepath"

	"studio/internal/filelock"
)

// ErrHeld means another process holds a record.
var ErrHeld = errors.New("the record is held by another program")

// HoldPath is the file locked by the process that holds a record.
func (s Store) HoldPath(id string) string { return filepath.Join(s.Dir, id+".hold") }

// Hold takes a record for the calling process, so that two programs never act
// on one record at once, such as the desktop app and the command line running
// one flow run. It fails with ErrHeld when another holder has it, including
// another Hold in this process. The lock is the operating system's own file
// lock, which it releases when the process ends, so a holder that crashes
// leaves nothing to clear. The file itself is never removed: removing a lock
// file lets a second holder lock a new file while the first still holds the
// old one.
func (s Store) Hold(id string) (release func() error, err error) {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(s.HoldPath(id), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	locked, err := filelock.TryLock(f)
	if err != nil || !locked {
		f.Close()
		if err == nil {
			err = ErrHeld
		}
		return nil, err
	}
	return func() error { return errors.Join(filelock.Unlock(f), f.Close()) }, nil
}

// Held reports whether a holder has a record now.
func (s Store) Held(id string) bool {
	f, err := os.OpenFile(s.HoldPath(id), os.O_RDWR, 0)
	if err != nil {
		return false
	}
	defer f.Close()
	locked, err := filelock.TryLock(f)
	if err != nil {
		return false
	}
	if locked {
		_ = filelock.Unlock(f)
	}
	return !locked
}
