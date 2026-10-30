package filelock

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// tryLock locks the first byte of a file exclusively without waiting.
// LockFileEx documents ERROR_LOCK_VIOLATION as the failure when another
// handle holds the range, and that a process's locks are released when it
// ends.
func tryLock(f *os.File) (bool, error) {
	ol := new(windows.Overlapped)
	err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ol)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return false, nil
	}
	return err == nil, err
}

func unlock(f *os.File) error {
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, new(windows.Overlapped))
}
