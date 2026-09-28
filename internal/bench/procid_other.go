//go:build unix && !linux

package bench

import (
	"errors"
	"syscall"
)

// processIdentity is empty on every platform but Windows and Linux. Apple
// documents neither a process start time nor a machine identifier through an
// interface Dinah can call without IOKit headers, so a record from here names
// its PID and its host alone.
func processIdentity() string {
	return ""
}

// sameProcessTable is rule 5 of the verdict here: a record whose host differs
// from this process's names a PID in another machine's table.
func sameProcessTable(record LockRecord, host, _ string) Verdict {
	if record.Host != host {
		return VerdictUnknown
	}
	return verdictContinue
}

// recordedProcessGone is rule 6 of the verdict here. POSIX documents the null
// signal as performing error checking without sending anything, and ESRCH as
// "no process or process group can be found"; ESRCH is gone and anything else
// is not.
func recordedProcessGone(record LockRecord) bool {
	err := syscall.Kill(record.PID, 0)
	return errors.Is(err, syscall.ESRCH)
}
