//go:build !windows

package savelock

import "os"

// openLockFile opens the lock file, making it if it is not there.
func openLockFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
}
