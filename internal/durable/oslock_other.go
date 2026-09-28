//go:build unix && !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package durable

import (
	"errors"
	"os"
)

// errNoOSLock is what tryOSLock answers on a Unix for which this package
// takes no documented whole-file lock, so an acquisition there proceeds
// without one and records os_lock false, and its staleness is never proven.
var errNoOSLock = errors.New("this platform offers no operating-system lock Dinah uses")

// tryOSLock answers errNoOSLock.
func tryOSLock(*os.File) (bool, error) {
	return false, errNoOSLock
}

// unlockOSLock has nothing to give back.
func unlockOSLock(*os.File) {}
