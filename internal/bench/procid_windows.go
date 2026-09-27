//go:build windows

package bench

import (
	"errors"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// processIdentity is this process's creation time from GetProcessTimes, as a
// decimal count of 100-nanosecond intervals since 1601-01-01 UTC. It is an
// absolute time, so after a reboot a recorded PID is either absent or belongs
// to a process created later.
func processIdentity() string {
	created, ok := creationTime(windows.CurrentProcess())
	if !ok {
		return ""
	}
	return strconv.FormatUint(created, 10)
}

// creationTime reads a process's creation time through a handle open on it.
func creationTime(process windows.Handle) (uint64, bool) {
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(process, &creation, &exit, &kernel, &user); err != nil {
		return 0, false
	}
	return uint64(creation.HighDateTime)<<32 | uint64(creation.LowDateTime), true
}

// sameProcessTable is rule 5 of the verdict on Windows: a record whose host
// differs from this process's names a PID in another machine's table.
func sameProcessTable(record LockRecord, host, _ string) Verdict {
	if record.Host != host {
		return VerdictUnknown
	}
	return verdictContinue
}

// recordedProcessGone is rule 6 of the verdict on Windows. A snapshot from
// CreateToolhelp32Snapshot that lacks the PID is gone. A PID present is opened
// with OpenProcess: a failure to open is not gone, a creation time from
// GetProcessTimes that differs from the record is gone, and the same creation
// time is gone only when WaitForSingleObject answers WAIT_OBJECT_0, since a
// process object is signalled when the process terminates. A failed snapshot
// is not gone.
func recordedProcessGone(record LockRecord) bool {
	identity := strings.TrimPrefix(record.Start, "windows:")
	recorded, err := strconv.ParseUint(identity, 10, 64)
	if err != nil {
		return false
	}
	present, ok := inSnapshot(uint32(record.PID))
	if !ok {
		return false
	}
	if !present {
		return true
	}
	access := uint32(windows.PROCESS_QUERY_LIMITED_INFORMATION | windows.SYNCHRONIZE)
	process, err := windows.OpenProcess(access, false, uint32(record.PID))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(process)
	created, ok := creationTime(process)
	if !ok {
		return false
	}
	if created != recorded {
		return true
	}
	event, err := windows.WaitForSingleObject(process, 0)
	if err != nil {
		return false
	}
	return event == windows.WAIT_OBJECT_0
}

// inSnapshot walks a process snapshot with Process32First and Process32Next
// and reports whether pid is in it. The second answer is false when the
// snapshot could not be taken or walked.
func inSnapshot(pid uint32) (bool, bool) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false, false
	}
	defer windows.CloseHandle(snapshot)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return false, false
	}
	for {
		if entry.ProcessID == pid {
			return true, true
		}
		err := windows.Process32Next(snapshot, &entry)
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return false, true
		}
		if err != nil {
			return false, false
		}
	}
}
