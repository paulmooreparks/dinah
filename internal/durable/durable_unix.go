//go:build unix

package durable

import (
	"errors"
	"io/fs"
	"os"
)

// POSIX rename(2), unlink(2) and open(2) do not fail because another process
// holds the file open, so no operation here is ever tried twice.

// openRead opens a file for reading.
func openRead(path string) (*os.File, error) {
	return os.Open(path)
}

// openDirOnce opens a directory, or a file, for reading. An open directory
// refuses no other process's rename or removal outside Windows.
func openDirOnce(path string) (*os.File, error) {
	return os.Open(path)
}

// openRetryable accepts nothing outside Windows.
func openRetryable(error) bool {
	return false
}

// replaceRetryable accepts nothing outside Windows.
func replaceRetryable(string) func(error) bool {
	return openRetryable
}

// moveRetryable accepts nothing outside Windows.
func moveRetryable(error) bool {
	return false
}

// removeAllRetryable accepts nothing outside Windows.
func removeAllRetryable(error) bool {
	return false
}

// createRetryable accepts nothing outside Windows.
func createRetryable(error) bool {
	return false
}

// replaceOnce renames a file over another with rename(2).
func replaceOnce(from, to string) error {
	observe("replace", to, 0)
	return os.Rename(from, to)
}

// moveOnce renames a directory with rename(2). MoveDir has already refused a
// target that exists, since rename(2) replaces an empty directory.
func moveOnce(from, to string) error {
	observe("move", to, 0)
	return os.Rename(from, to)
}

// removeOnce removes one file with unlink(2).
func removeOnce(path string) error {
	return os.Remove(path)
}

// openJournalOnce opens a journal with O_APPEND, which open(2) documents as
// positioning each write at the end of the file, and answers whether the open
// created it. The exclusive attempt comes first so that the answer is exact.
func openJournalOnce(path string) (*os.File, bool, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE|os.O_EXCL, 0o644)
	if err == nil {
		return f, true, nil
	}
	if !errors.Is(err, fs.ErrExist) {
		return nil, false, err
	}
	f, err = os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644)
	return f, false, err
}

// appendAtEnd writes data in one call on a handle opened with O_APPEND.
func appendAtEnd(f *os.File, data []byte) error {
	_, err := f.Write(data)
	return err
}

// createExclusiveOnce creates a lock file with O_CREAT|O_EXCL, which open(2)
// documents as failing with EEXIST when the name exists.
func createExclusiveOnce(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644)
}

// openLockFileOnce opens an existing lock file for reading and writing.
func openLockFileOnce(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDWR, 0)
}

// deleteHeld removes the lock file's name.
func deleteHeld(_ *os.File, path string) error {
	return os.Remove(path)
}

// stillNamed reports whether the handle and the name reach the same file,
// which the os package defines on Unix as the same device and inode.
func stillNamed(f *os.File, path string) (bool, error) {
	held, err := f.Stat()
	if err != nil {
		return false, err
	}
	named, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return os.SameFile(held, named), nil
}
