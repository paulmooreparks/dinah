// Package statefile reads and writes Dinah's small JSON state files,
// such as settings.json and recent-documents.json, so that an interrupted
// write never leaves a file half written. A write first records both complete
// versions, the one it replaces and the new one, in <file>.pending, then
// replaces the file and removes the record; a read or a write that finds the
// record finishes the replacement first. This does not depend on a rename
// being atomic on Windows, and a file changed outside Dinah while a
// record is pending stops the recovery rather than being overwritten.
//
// Read and Write serialise the calls of one program. A file that both the
// desktop app and the command line write is changed through Update and read
// through ReadShared, never Read or Write: those two also take the file's
// lock, the operating system's lock on <file>.lock, so the two programs never
// change it at once.
package statefile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"studio/internal/filelock"
	"studio/internal/profile"
)

var mu sync.Mutex

type transaction struct {
	Version  int    `json:"version"`
	Previous []byte `json:"previous"`
	Next     []byte `json:"next"`
}

// Directory makes root, and refuses it when it is not a real directory, such
// as a symbolic link to one.
func Directory(root string) error {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s must be a real directory", root)
	}
	return nil
}

// Publish creates a new name for a fully written, closed file. Link refuses
// an existing destination, so Publish never replaces another file.
func Publish(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".state-copy-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Link(tmp.Name(), path)
}

// ReadRegular reads a file, refusing one that is not a regular file, such as
// a symbolic link.
func ReadRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular state file", path)
	}
	return os.ReadFile(path)
}

func replace(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".state-write-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp.Name(), path)
}

// finish finishes a pending replacement; the caller holds mu.
func finish(path string) error {
	data, err := ReadRegular(path + ".pending")
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var tx transaction
	if json.Unmarshal(data, &tx) != nil || tx.Version != 1 || !json.Valid(tx.Next) {
		return fmt.Errorf("%s has an invalid pending state transaction", path)
	}
	current, err := ReadRegular(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil && !bytes.Equal(current, tx.Previous) && !bytes.Equal(current, tx.Next) {
		return fmt.Errorf("%s changed outside %s during a pending save; preserve both versions before resolving it", path, profile.Current().Name)
	}
	if !bytes.Equal(current, tx.Next) {
		if err := replace(path, tx.Next); err != nil {
			return err
		}
	}
	return os.Remove(path + ".pending")
}

func read(path string) ([]byte, error) {
	if err := finish(path); err != nil {
		return nil, err
	}
	return ReadRegular(path)
}

func write(path string, data []byte, validatePrevious func([]byte) error) error {
	if !json.Valid(data) {
		return errors.New("state must be valid JSON")
	}
	if err := Directory(filepath.Dir(path)); err != nil {
		return err
	}
	if err := finish(path); err != nil {
		return err
	}
	previous, err := ReadRegular(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if previous != nil && !json.Valid(previous) {
		return fmt.Errorf("%s contains invalid JSON and has not been replaced", path)
	}
	if previous != nil && validatePrevious != nil {
		if err := validatePrevious(previous); err != nil {
			return err
		}
	}
	tx, err := json.Marshal(transaction{Version: 1, Previous: previous, Next: data})
	if err != nil {
		return err
	}
	if err := Publish(path+".pending", tx); err != nil {
		return err
	}
	return finish(path)
}

// Read returns a state file's content, finishing a pending write first. A
// missing file is reported as os.ErrNotExist.
func Read(path string) ([]byte, error) {
	mu.Lock()
	defer mu.Unlock()
	return read(path)
}

// Write replaces a state file with data, which must be valid JSON.
func Write(path string, data []byte) error {
	return WriteValidated(path, data, nil)
}

// WriteValidated replaces a state file with data, provided validatePrevious,
// when given, accepts the content it replaces.
func WriteValidated(path string, data []byte, validatePrevious func([]byte) error) error {
	mu.Lock()
	defer mu.Unlock()
	return write(path, data, validatePrevious)
}

// LockWait bounds how long ReadShared and Update wait for another holder of
// a file's lock. Tests shorten it.
var LockWait = 5 * time.Second

// lock takes the file's lock: the operating system's exclusive lock on
// <file>.lock (package filelock), which the system releases when a holder's
// process ends, so a crash leaves nothing to clear. Another open of the lock
// file, in this program or another, waits for it. The lock files stay.
//
// Two things keep one writer from starving the others, which polling for an
// exclusive lock does not do by itself: a writer that releases the lock and
// at once asks again would win every time, and the others would wait for its
// whole run. Writers of this program wait their turn in a queue of their own,
// in order, and only the one at the front waits on the operating system, so
// LockWait is only ever spent waiting for another program. Between programs,
// a writer takes a second lock, <file>.gate, before the data lock and keeps
// it until it has the data lock, so a program that has just released the data
// lock finds the gate held by the waiter and must wait behind it. Both locks
// are exclusive file locks, which mean only what they say; nothing here
// assumes the system serves waiters in order.
func lock(path string) (func(), error) {
	if err := Directory(filepath.Dir(path)); err != nil {
		return nil, err
	}
	leave := enter(filepath.Clean(path))
	unlock, err := lockFiles(path)
	if err != nil {
		leave()
		return nil, err
	}
	return func() { unlock(); leave() }, nil
}

// lockFiles takes the gate and then the data lock, within one LockWait.
func lockFiles(path string) (func(), error) {
	deadline := time.Now().Add(LockWait)
	gate, err := lockOne(path, path+".gate", deadline)
	if err != nil {
		return nil, err
	}
	data, err := lockOne(path, path+".lock", deadline)
	gate()
	return data, err
}

// lockOne takes the exclusive lock on one lock file, trying again every
// 20 ms until the deadline.
func lockOne(path, name string, deadline time.Time) (func(), error) {
	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("%s could not be locked: %w", filepath.Base(path), err)
	}
	for {
		locked, err := filelock.TryLock(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("%s could not be locked: %w", filepath.Base(path), err)
		}
		if locked {
			return func() { _ = filelock.Unlock(f); f.Close() }, nil
		}
		if time.Now().After(deadline) {
			f.Close()
			return nil, fmt.Errorf("%s is being changed by another program; try again", filepath.Base(path))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// queues has one queue for each lock file that this program's writers are
// using or waiting for.
var queues = struct {
	sync.Mutex
	files map[string]*queue
}{files: map[string]*queue{}}

type queue struct {
	held    bool
	waiters []chan struct{}
	refs    int // the holder and the waiters
}

// enter waits for this program's turn at a file's lock, in the order writers
// asked, with no deadline of its own since everyone ahead is making progress
// in the same program. The function it returns hands the turn to the next.
func enter(key string) func() {
	queues.Lock()
	q := queues.files[key]
	if q == nil {
		q = &queue{}
		queues.files[key] = q
	}
	q.refs++
	if !q.held {
		q.held = true
		queues.Unlock()
	} else {
		turn := make(chan struct{})
		q.waiters = append(q.waiters, turn)
		queues.Unlock()
		<-turn
	}
	return func() {
		queues.Lock()
		defer queues.Unlock()
		q.refs--
		if len(q.waiters) > 0 {
			next := q.waiters[0]
			q.waiters = q.waiters[1:]
			close(next)
			return
		}
		q.held = false
		if q.refs == 0 {
			delete(queues.files, key)
		}
	}
}

// ReadShared is Read for a file that another program also writes: it takes
// the file's lock, so it never finishes another program's write at the same
// moment as that program does. The program's own state files are not held
// up while it waits for the lock.
func ReadShared(path string) ([]byte, error) {
	return ReadSharedWithLock(path, path)
}

// ReadSharedWithLock is ReadShared for a file changed through
// UpdateWithLock, with the same lockPath.
func ReadSharedWithLock(path, lockPath string) ([]byte, error) {
	unlock, err := lock(lockPath)
	if err != nil {
		return nil, err
	}
	defer unlock()
	mu.Lock()
	defer mu.Unlock()
	return read(path)
}

// Update changes a file that another program also writes. Under the file's
// lock, change is given the current content, or nil when there is no file,
// and returns the new content, which replaces it. When change returns nil
// content and no error, nothing is written. A file that is changed through
// Update is read only through ReadShared, never Read or Write, which do not
// take the lock.
func Update(path string, change func(previous []byte) ([]byte, error)) error {
	return UpdateWithLock(path, path, change)
}

// UpdateWithLock is Update with the lock taken on lockPath's lock file
// rather than path's, for a file kept in a folder the person works in, such
// as a document's Working Material, where a lock file would be clutter. Every
// writer of the file must name the same lockPath.
func UpdateWithLock(path, lockPath string, change func(previous []byte) ([]byte, error)) error {
	unlock, err := lock(lockPath)
	if err != nil {
		return err
	}
	defer unlock()
	mu.Lock()
	defer mu.Unlock()
	previous, err := read(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	next, err := change(previous)
	if err != nil || next == nil {
		return err
	}
	return write(path, next, nil)
}
