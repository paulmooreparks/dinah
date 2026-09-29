// Package durable holds every open, write, rename, removal and append that
// Dinah performs on a workbench tree or on the setup ledger, together with the
// operating-system file lock an entity lock's holder keeps and the in-process
// registry of held locks. No other package opens, renames or removes a file
// on a workbench tree. guard_test.go in this package holds the module to that
// through the type checker: every package-level function of os, io/fs and
// io/ioutil and every method of os.Root is refused outside this package unless
// the guard permits it by name, and a list of functions is refused from
// syscall, golang.org/x/sys/windows and golang.org/x/sys/unix. The guard does
// not see a function of those three packages missing from its list, a
// third-party library opening a path Dinah hands it, or the four packages it
// leaves out with their reasons; the comment at the top of guard_test.go names
// each.
//
// Three properties live here so that no call site can forget them. A write
// reaches the disk before the rename that publishes it. Dinah's own readers
// open with every sharing flag on Windows, so a rename over a file Dinah is
// reading is not refused by Dinah itself. And an operation that another
// process's handle refuses on Windows is tried again, for a bounded time
// before an act has written anything and without a bound after it.
package durable

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// RetryBudget is how long an operation that may give up waits out a transient
// refusal before it does, and the interval at which an operation that may not
// give up repeats its notice. Tests shorten it; nothing else writes it.
var RetryBudget = 5 * time.Second

// The first and the longest pause between two attempts of a retried
// operation. The pause doubles from the first to the second.
const (
	firstPause = 10 * time.Millisecond
	lastPause  = 250 * time.Millisecond
)

// Wait describes an operation that may not give up and is still retrying.
type Wait struct {
	// Op is the operation, one of the names BusyError.Op carries.
	Op string
	// Path is the file the operating system keeps refusing.
	Path string
	// Last is the most recent error the operating system returned.
	Last error
	// Elapsed is how long the operation has been retrying.
	Elapsed time.Duration
	// Notice is 1 for the first notice, 2 for the second, and so on.
	Notice int
}

// Waiting is called each time another RetryBudget passes while an operation
// that may not give up retries. Every head sets it once when it starts, before
// it serves anything, and nothing else writes it.
var Waiting func(Wait)

// BusyError is what an operation that may give up answers when RetryBudget ran
// out while the operating system kept refusing it.
type BusyError struct {
	// Op is one of "open", "create", "write", "replace", "remove", "append"
	// and "move".
	Op string
	// Path is the file or directory the operation was refused on.
	Path string
	// Err is the last error the operating system returned.
	Err error
	// Structural is true for MoveDir and RemoveAll, which run as a
	// structural act's apply step or as cleanup, and whose refusal a
	// structural act reports as an interruption rather than as busy.
	Structural bool
}

// Error reports the operation, the path and the last error.
func (e *BusyError) Error() string {
	return e.Op + " " + e.Path + ": still refused after the retry budget: " + e.Err.Error()
}

// Unwrap answers the last error the operating system returned, so a caller
// can still test for its cause.
func (e *BusyError) Unwrap() error {
	return e.Err
}

// Step is one observed step of a durable operation.
type Step struct {
	// Op is one of "write", "sync", "close", "replace", "move", "syncdir",
	// "remove" and "append".
	Op string
	// Path is the file or directory the step acts on.
	Path string
	// Flags carries the MoveFileEx flags on a Windows "replace" or "move",
	// and is zero everywhere else.
	Flags uint32
}

// Observe, when set, is called before each step of a durable operation. A
// non-nil answer on a "sync" step is returned as though Sync had failed, and an
// answer on any other step is ignored. Only tests set it.
var Observe func(Step) error

// observe reports one step to Observe and answers what it said.
func observe(op, path string, flags uint32) error {
	if Observe == nil {
		return nil
	}
	return Observe(Step{Op: op, Path: path, Flags: flags})
}

// retry runs attempt until it succeeds, fails with an error retryable does not
// accept, or, for a class that may give up, RetryBudget has passed. A class
// that may not give up calls Waiting each time another RetryBudget passes.
func retry(op, path string, c class, retryable func(error) bool, attempt func() error) error {
	start := time.Now()
	pause := firstPause
	notices := 0
	for {
		err := attempt()
		if err == nil {
			c.done()
			return nil
		}
		if !retryable(err) {
			return err
		}
		elapsed := time.Since(start)
		if c.mayGive && elapsed >= RetryBudget {
			return &BusyError{Op: op, Path: path, Err: err}
		}
		if !c.mayGive {
			for elapsed >= time.Duration(notices+1)*RetryBudget {
				notices++
				notify(op, path, err, elapsed, notices)
			}
		}
		wait := pause
		if c.mayGive && RetryBudget-elapsed < wait {
			wait = RetryBudget - elapsed
		}
		time.Sleep(wait)
		pause = min(pause*2, lastPause)
	}
}

// notify hands one notice to Waiting, when a head has set it.
func notify(op, path string, last error, elapsed time.Duration, notice int) {
	if Waiting == nil {
		return
	}
	Waiting(Wait{Op: op, Path: path, Last: last, Elapsed: elapsed, Notice: notice})
}

// Open opens a file for reading. On Windows the handle shares read, write and
// delete access, so Dinah's own reading never refuses Dinah's own rename,
// append or removal.
func Open(path string) (*os.File, error) {
	c := classify(false)
	var f *os.File
	err := retry("open", path, c, openRetryable, func() error {
		var err error
		f, err = openRead(path)
		return err
	})
	return f, err
}

// OpenDir opens a directory, or a file, for reading, so its caller can stat it
// and list it through one handle with (*os.File).ReadDir. On Windows the
// handle shares read, write and delete, as Open's does, so a listing never
// refuses another process's rename or removal of the directory while it runs.
func OpenDir(path string) (*os.File, error) {
	c := classify(false)
	var f *os.File
	err := retry("open", path, c, openRetryable, func() error {
		var err error
		f, err = openDirOnce(path)
		return err
	})
	return f, err
}

// ReadFile reads a whole file through Open, sizing its buffer from the file's
// length the way os.ReadFile does, and reading on to the end in case the file
// grew or reports no length, as a file under /proc does.
func ReadFile(path string) ([]byte, error) {
	f, err := Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	size := 512
	if info, err := f.Stat(); err == nil && info.Size() > 0 && info.Size() < 1<<30 {
		size = int(info.Size()) + 1
	}
	data := make([]byte, 0, size)
	for {
		n, err := f.Read(data[len(data):cap(data)])
		data = data[:len(data)+n]
		if errors.Is(err, io.EOF) {
			return data, nil
		}
		if err != nil {
			return data, err
		}
		if len(data) == cap(data) {
			data = append(data, 0)[:len(data)]
		}
	}
}

// WriteFile replaces the file at path with data. The bytes go to a temporary
// beside the destination, are flushed to stable storage, and only then renamed
// over the destination, so after a crash the name holds either the old bytes
// or the new ones and never a file whose data did not reach the disk. On Linux
// the directory is flushed after the rename as well.
//
// A failure before the rename removes the temporary and leaves the
// destination untouched.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".dinah-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if err := fillTemporary(tmp, data, perm); err != nil {
		os.Remove(name)
		return err
	}
	c := classify(true)
	if err := replaceClassed("write", name, path, c); err != nil {
		os.Remove(name)
		return err
	}
	return syncDir(dir)
}

// fillTemporary writes, flushes and closes a temporary, reporting each step to
// Observe. It closes the file on every path.
func fillTemporary(tmp *os.File, data []byte, perm os.FileMode) error {
	name := tmp.Name()
	observe("write", name, 0)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := syncFile(tmp); err != nil {
		tmp.Close()
		return err
	}
	observe("close", name, 0)
	return tmp.Close()
}

// syncFile flushes a file to stable storage, reporting the step to Observe
// first and returning Observe's answer in Sync's place when it is not nil.
func syncFile(f *os.File) error {
	if err := observe("sync", f.Name(), 0); err != nil {
		return err
	}
	if skipsFlush(f.Name()) {
		return nil
	}
	return f.Sync()
}

// Replace renames a file over an existing file, or over nothing. On Linux the
// directory is flushed afterwards.
func Replace(from, to string) error {
	c := classify(true)
	if err := replaceClassed("replace", from, to, c); err != nil {
		return err
	}
	return syncDir(filepath.Dir(to))
}

// replaceClassed is Replace's rename inside the retry, reported as op.
func replaceClassed(op, from, to string, c class) error {
	return retry(op, to, c, replaceRetryable(to), func() error {
		return replaceOnce(from, to)
	})
}

// MoveDir renames a directory, or a file, to a path that must not exist. It
// gives up once RetryBudget has passed wherever it falls, because it runs only
// as a structural act's apply step or as cleanup, and a structural act reports
// an interruption that dinah check --finish completes. On Linux the parent
// directories of both paths are flushed afterwards.
func MoveDir(from, to string) error {
	if _, err := os.Lstat(to); err == nil {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: fs.ErrExist}
	}
	err := retry("move", to, alwaysGiveUp, moveRetryable, func() error {
		return moveOnce(from, to)
	})
	if err != nil {
		return structural(err)
	}
	if err := syncDir(filepath.Dir(from)); err != nil {
		return err
	}
	return syncDir(filepath.Dir(to))
}

// Remove removes one file. An absent file is reported as the error the
// operating system gives for it, which callers test with errors.Is and
// fs.ErrNotExist.
func Remove(path string) error {
	c := classify(true)
	observe("remove", path, 0)
	return retry("remove", path, c, replaceRetryable(path), func() error {
		return removeOnce(path)
	})
}

// RemoveLock removes a lock file on release. It gives up once RetryBudget has
// passed and never reads or sets any entry's Wrote, because a lock's removal
// is not part of the act the lock protected.
func RemoveLock(path string) error {
	observe("remove", path, 0)
	return retry("remove", path, alwaysGiveUp, replaceRetryable(path), func() error {
		return removeOnce(path)
	})
}

// RemoveAll removes a tree. It gives up once RetryBudget has passed wherever
// it falls, on the terms MoveDir does.
func RemoveAll(path string) error {
	observe("remove", path, 0)
	err := retry("remove", path, alwaysGiveUp, removeAllRetryable, func() error {
		return os.RemoveAll(path)
	})
	return structural(err)
}

// structural marks a *BusyError from MoveDir or RemoveAll as structural and
// answers every other error unchanged.
func structural(err error) error {
	var busy *BusyError
	if errors.As(err, &busy) {
		busy.Structural = true
	}
	return err
}

// OpenJournal opens a journal for reading and writing, creating it when it is
// absent. Appends made through AppendLine land at the end of the file.
func OpenJournal(path string) (*os.File, error) {
	c := classify(true)
	f, _, err := openJournalClassed(path, c)
	return f, err
}

// openJournalClassed is OpenJournal inside the retry for a class the caller
// has already read, answering whether the open created the file.
func openJournalClassed(path string, c class) (*os.File, bool, error) {
	var f *os.File
	var created bool
	err := retry("append", path, c, replaceRetryable(path), func() error {
		var err error
		f, created, err = openJournalOnce(path)
		return err
	})
	return f, created, err
}

// AppendLine appends one line and its newline to a journal in a single write
// at the end of the file, flushes it, and closes the file. When the open
// created the journal on Linux, the directory is flushed as well. A write or a
// flush that fails cuts the file back to the length it had before, so a
// failed append leaves the journal as it found it. Every append to a journal
// is made under the lock of the entity the journal belongs to, which
// bench.AppendEvent refuses to do without, so nothing can have landed after
// the failed line.
func AppendLine(path string, line []byte) error {
	c := classify(true)
	unclassed := c
	unclassed.mutating = false
	f, created, err := openJournalClassed(path, unclassed)
	if err != nil {
		return err
	}
	whole := make([]byte, 0, len(line)+1)
	whole = append(whole, line...)
	whole = append(whole, '\n')
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	before := info.Size()
	observe("append", path, 0)
	if err := appendAtEnd(f, whole); err != nil {
		f.Truncate(before)
		f.Close()
		return err
	}
	if err := syncFile(f); err != nil {
		f.Truncate(before)
		f.Close()
		return err
	}
	observe("close", path, 0)
	if err := f.Close(); err != nil {
		return err
	}
	c.done()
	if !created {
		return nil
	}
	return syncDir(filepath.Dir(path))
}

// Truncate cuts a journal back to size bytes and flushes it. It is the one
// shortening of a journal Dinah performs, which is the tail repair's cut back
// to the journal's last whole line, made by the holder of the journal's lock
// after the fragment it removes has been written somewhere else and flushed.
func Truncate(path string, size int64) error {
	c := classify(true)
	unclassed := c
	unclassed.mutating = false
	f, _, err := openJournalClassed(path, unclassed)
	if err != nil {
		return err
	}
	observe("truncate", path, 0)
	if err := f.Truncate(size); err != nil {
		f.Close()
		return err
	}
	if err := syncFile(f); err != nil {
		f.Close()
		return err
	}
	observe("close", path, 0)
	if err := f.Close(); err != nil {
		return err
	}
	c.done()
	return nil
}

// SamePath reports whether two paths name one file in the form lock paths are
// compared in: cleaned, and folded to lower case on the platforms whose file
// systems compare names without regard to case by default.
func SamePath(a, b string) bool {
	return pathKey(a) == pathKey(b)
}

// CreateExclusive creates a lock file that must not already exist, opened for
// reading and writing. An existing file answers an error for which
// errors.Is(err, fs.ErrExist) holds, at once. On Windows a denial is tried
// again until RetryBudget has passed, because a released lock whose deletion
// is still pending answers one.
func CreateExclusive(path string) (*os.File, error) {
	var f *os.File
	err := retry("create", path, alwaysGiveUp, createRetryable, func() error {
		var err error
		f, err = createExclusiveOnce(path)
		return err
	})
	return f, err
}

// OpenLockFile opens an existing lock file for reading and writing, with the
// access its holder needs to delete it on release. An absent file answers an
// error for which errors.Is(err, fs.ErrNotExist) holds.
//
// A denial is tried again until RetryBudget has passed, as CreateExclusive
// tries it, because a lock released while another handle still has it open
// answers one on Windows until that handle closes, and the retry then meets
// the file gone.
func OpenLockFile(path string) (*os.File, error) {
	var f *os.File
	retryable := func(err error) bool { return openRetryable(err) || createRetryable(err) }
	err := retry("open", path, alwaysGiveUp, retryable, func() error {
		var err error
		f, err = openLockFileOnce(path)
		return err
	})
	return f, err
}

// TryOSLock takes the exclusive operating-system lock on a lock file's handle
// without waiting. It answers false and no error when another handle holds
// the lock, and an error when the file system offers no such lock.
func TryOSLock(f *os.File) (bool, error) {
	return tryOSLock(f)
}

// TakeOSLock takes the exclusive operating-system lock on a lock file its
// caller has just created. A judge may hold that lock for the moment it takes
// to read an empty record, so a refusal is tried again until RetryBudget has
// passed, and then answered as a *BusyError. It answers false and no error
// when the file system offers no such lock, and the caller then holds the
// lock without one.
func TakeOSLock(f *os.File) (bool, error) {
	taken := false
	refused := errors.New("the operating-system lock is held by another handle")
	err := retry("create", f.Name(), alwaysGiveUp, func(err error) bool { return err == refused }, func() error {
		ok, err := tryOSLock(f)
		if err != nil {
			return nil
		}
		if !ok {
			return refused
		}
		taken = true
		return nil
	})
	return taken, err
}

// WriteHeld replaces the whole content of a lock file through its holder's
// own handle and flushes it, so the record is on the disk before the
// acquisition that wrote it returns.
func WriteHeld(f *os.File, data []byte) error {
	if err := f.Truncate(0); err != nil {
		return err
	}
	observe("write", f.Name(), 0)
	if _, err := f.WriteAt(data, 0); err != nil {
		return err
	}
	return syncFile(f)
}

// ReadHeld reads the whole content of a lock file through a handle already
// open on it.
func ReadHeld(f *os.File) ([]byte, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	data := make([]byte, info.Size())
	n, err := f.ReadAt(data, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return data[:n], nil
}

// DeleteHeld removes the lock file its holder has open. On Windows it marks
// the file the handle refers to for deletion, so the deletion follows the file
// rather than the name; elsewhere it removes the name.
func DeleteHeld(f *os.File, path string) error {
	return deleteHeld(f, path)
}

// CloseLockFile gives up the operating-system lock on a lock file's handle
// and closes it. The Windows documentation recommends unlocking explicitly
// rather than leaving the unlock to the close, whose timing it leaves open.
func CloseLockFile(f *os.File) error {
	unlockOSLock(f)
	return f.Close()
}

// StillNamed reports whether a lock file's handle still refers to the file
// that path names: on Windows, that its deletion is not pending; elsewhere,
// that the handle and the name reach the same device and inode.
func StillNamed(f *os.File, path string) (bool, error) {
	return stillNamed(f, path)
}
