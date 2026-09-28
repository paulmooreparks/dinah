//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package durable

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// tryOSLock takes flock(2) with LOCK_EX|LOCK_NB. The Linux flock(2) page
// documents that the lock belongs to the open file description, is released
// when every descriptor referring to it has closed, and that descriptors from
// separate open calls "are treated independently". A refusal is EWOULDBLOCK.
func tryOSLock(f *os.File) (bool, error) {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, unix.EWOULDBLOCK) {
		return false, nil
	}
	return false, err
}

// unlockOSLock gives the flock back before the close that would release it
// anyway.
func unlockOSLock(f *os.File) {
	unix.Flock(int(f.Fd()), unix.LOCK_UN)
}
