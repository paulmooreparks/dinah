// Package savelock arbitrates the replacing of a file between programs
// built from this source: Dinah, a product built from it, and a document
// application of its own (docs/proposals/standalone-document-apps.md, The
// save contract). Each checks that a file still has the revision its text
// was based on and then renames the new text over it. Two processes could
// both pass that check before either renamed, and the later rename then
// replaced the earlier text with no conflict reported.
//
// A writer holds the lock for a target from before its last check until its
// rename is done. The lock is the operating system's exclusive lock on a
// lock file (internal/filelock: LockFileEx on Windows, flock elsewhere),
// which excludes a second holder in any process and is released when its
// holder ends. Every such program finds the same lock file, because it is in
// one folder for the person, outside any product's own folder, and its name
// comes from the identity of the target's folder. One lock therefore stands
// for a whole folder, and the saves of two files in one folder wait for each
// other. That costs a moment and needs no rule for when two file names are
// the same file, which file systems decide differently and Windows does not
// document as a comparison a program can make.
//
// A program that knows nothing of this lock is not held by it. Its change is
// seen when it lands before the last check and is replaced unseen when it
// lands between that check and the rename, which no file interface that
// Windows, Linux and macOS all document prevents.
package savelock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"studio/internal/filelock"
	"studio/internal/profile"
)

// ErrBusy is returned when another program holds a folder's lock for longer
// than the caller would wait. The caller has seen no conflict and has written
// nothing.
var ErrBusy = errors.New("a file in this folder is being saved by another program; save again in a moment")

// ErrNoIdentity is returned when the identity of a target's folder could not
// be read. The save is refused: a writer that guessed a name for the lock
// could choose another than a second writer does, and the two would then not
// exclude each other while each believed they did.
var ErrNoIdentity = errors.New("the folder's identity could not be read, so two programs saving here could not be kept apart; the file was not saved and your text is kept")

// Wait is how long a save waits for another program's save in the same
// folder. A save holds the lock for one comparison and one rename.
const Wait = 5 * time.Second

// retry is how often a waiting writer tries again.
const retry = 20 * time.Millisecond

// identityOf reads a folder's identity. A test replaces it.
var identityOf = folderIdentity

// dir is the folder the lock files are in. It is the person's, and the same
// for every product built from this source: the operating system's folder
// for a person's application data (os.UserConfigDir) and a name from the
// source product's own. It is not a cache folder, which the system or a
// tidying tool may empty: a lock file removed while its holder is still at
// work would let a second writer lock a new file of the same name. Nothing
// here removes a lock file. There is one for each folder a document was
// ever saved in, and each is empty. A test's locks go under the temporary
// folder, so a test never waits on a program the person is running.
func dir() (string, error) {
	name := profile.Source().ID + "-save-locks"
	if testing.Testing() {
		return filepath.Join(os.TempDir(), name+"-test"), nil
	}
	data, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(data, name), nil
}

// key is the lock file's name for a target's folder: a digest of the
// folder's identity as the operating system gives it (identity_windows.go,
// identity_unix.go), so two paths to one folder give one key. A file's own
// identity is no use, since a rename gives the target a new one at every
// save.
//
// Any failure to read the identity refuses the save (ErrNoIdentity). The
// folder's path does not stand in for it: nothing documented says that two
// paths to one folder resolve to the same text on every file system, and a
// writer cannot tell that they did not.
func key(folder string) (string, error) {
	id, err := identityOf(folder)
	if err != nil || len(id) == 0 {
		return "", ErrNoIdentity
	}
	sum := sha256.Sum256(id)
	return hex.EncodeToString(sum[:]) + ".lock", nil
}

// Hold takes the lock for the files of folder, waiting up to wait for
// another holder, and returns the function that releases it. It returns
// ErrBusy when the wait runs out, ErrNoIdentity when the folder's identity
// cannot be read, and the context's error when it is done.
func Hold(ctx context.Context, folder string, wait time.Duration) (release func(), err error) {
	d, err := dir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(d, 0o700); err != nil {
		return nil, err
	}
	name, err := key(folder)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(d, name)
	deadline := time.Now().Add(wait)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		f, held, err := try(path)
		if err != nil {
			return nil, err
		}
		if held {
			return func() { _ = filelock.Unlock(f); f.Close() }, nil
		}
		if !time.Now().Before(deadline) {
			return nil, ErrBusy
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retry):
		}
	}
}

// try opens the lock file and takes its lock without waiting. On Windows the
// file is opened so that it cannot be removed while it is open
// (open_windows.go). Elsewhere a file can be removed while it is open, and a
// lock on a file that is no longer the one at the path excludes nobody who
// opens the path afterwards, so the lock counts only when the file locked is
// still the file there. A lock file removed after that, while its holder
// works, is not seen there. Nothing built from this source removes one, and
// the folder is not one the system tidies.
func try(path string) (f *os.File, held bool, err error) {
	f, err = openLockFile(path)
	if err != nil {
		return nil, false, err
	}
	locked, err := filelock.TryLock(f)
	if err != nil || !locked {
		f.Close()
		return nil, false, err
	}
	mine, err1 := f.Stat()
	there, err2 := os.Stat(path)
	if err1 != nil || err2 != nil || !os.SameFile(mine, there) {
		_ = filelock.Unlock(f)
		f.Close()
		return nil, false, nil
	}
	return f, true, nil
}
