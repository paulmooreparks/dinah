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
// comes from the identity of the target's folder and the target's name.
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
	"strings"
	"testing"
	"time"

	"studio/internal/filelock"
	"studio/internal/profile"
)

// ErrBusy is returned when another program holds a target's lock for longer
// than the caller would wait. The caller has seen no conflict and has written
// nothing.
var ErrBusy = errors.New("the file is being saved by another program; save again in a moment")

// Wait is how long a save waits for another program's save of the same file.
// A save holds the lock for one comparison and one rename.
const Wait = 5 * time.Second

// retry is how often a waiting writer tries again.
const retry = 20 * time.Millisecond

// dir is the folder the lock files are in. It is the person's, and the same
// for every product built from this source: the operating system's cache
// folder for the person (os.UserCacheDir) and a name from the source
// product's own. A test's locks go under the temporary folder, so a test
// never waits on a program the person is running.
func dir() (string, error) {
	name := profile.Source().ID + "-save-locks"
	if testing.Testing() {
		return filepath.Join(os.TempDir(), name+"-test"), nil
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, name), nil
}

// key is the lock file's name for a target: a digest of its folder's
// identity and its own name. The folder's identity is the operating system's
// (identity_windows.go, identity_unix.go), so two paths to one folder give
// one key. The file's own identity is no use, since a rename gives the
// target a new one at every save. The name is compared without regard to
// case. In a folder that tells names apart by case, two files that differ
// only in case therefore share a lock, which makes their saves wait for each
// other and loses nothing.
//
// Where the file system gives a folder no identity, the folder's path with
// its links resolved stands in for it. Two paths to such a folder that
// resolve differently are then not arbitrated, and the save is as it was
// before this package.
func key(folder, name string) string {
	id, err := folderIdentity(folder)
	if err != nil || len(id) == 0 {
		resolved, rerr := filepath.EvalSymlinks(folder)
		if rerr != nil {
			resolved = folder
		}
		if abs, aerr := filepath.Abs(resolved); aerr == nil {
			resolved = abs
		}
		id = []byte("path\x00" + strings.ToLower(filepath.Clean(resolved)))
	}
	h := sha256.New()
	h.Write(id)
	h.Write([]byte{0})
	h.Write([]byte(strings.ToLower(name)))
	return hex.EncodeToString(h.Sum(nil)) + ".lock"
}

// Hold takes the lock for the file called name in folder, waiting up to wait
// for another holder, and returns the function that releases it. It returns
// ErrBusy when the wait runs out and the context's error when it is done.
func Hold(ctx context.Context, folder, name string, wait time.Duration) (release func(), err error) {
	d, err := dir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(d, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(d, key(folder, name))
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

// try opens the lock file and takes its lock without waiting. A lock file may
// be removed by whatever tidies the cache folder. A lock on a file that is no
// longer the one at the path excludes nobody who opens the path afterwards,
// so the lock counts only when the file locked is still the file there.
func try(path string) (f *os.File, held bool, err error) {
	f, err = os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
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
