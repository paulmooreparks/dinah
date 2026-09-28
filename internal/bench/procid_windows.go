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

// processTable is what rule 6 asks of the operating system's process table,
// behind an interface so a test can answer for a PID without depending on
// which processes happen to be running.
type processTable interface {
	// present reports whether pid is in a process snapshot; ok is false when
	// the snapshot could not be taken or walked.
	present(pid uint32) (present, ok bool)
	// open opens the process with the access named.
	open(pid uint32, access uint32) (processHandle, error)
}

// processHandle is an open process.
type processHandle interface {
	// creation answers the process's creation time from GetProcessTimes.
	creation() (uint64, bool)
	// ended reports whether the process object is signalled, which it is once
	// the process has terminated.
	ended() (bool, error)
	close()
}

// processes is the table rule 6 reads. Tests replace it.
var processes processTable = liveProcesses{}

// recordedProcessGone is rule 6 of the verdict on Windows. A snapshot from
// CreateToolhelp32Snapshot that lacks the PID is gone, and a failed snapshot
// is not gone. A PID present is opened with PROCESS_QUERY_LIMITED_INFORMATION
// alone, which the GetProcessTimes documentation names as sufficient: a
// failure to open is not gone, and a creation time that differs from the
// record is gone, because the PID now names a later process. Only when the
// creation times agree is the process opened again with SYNCHRONIZE, which
// WaitForSingleObject needs and which a process of another user commonly
// refuses; that second handle's creation time is compared again, since the
// PID may have been reused between the two opens, and the same creation time
// is gone only when WaitForSingleObject answers WAIT_OBJECT_0.
func recordedProcessGone(record LockRecord) bool {
	identity := strings.TrimPrefix(record.Start, "windows:")
	recorded, err := strconv.ParseUint(identity, 10, 64)
	if err != nil {
		return false
	}
	pid := uint32(record.PID)
	present, ok := processes.present(pid)
	if !ok {
		return false
	}
	if !present {
		return true
	}
	query, err := processes.open(pid, windows.PROCESS_QUERY_LIMITED_INFORMATION)
	if err != nil {
		return false
	}
	created, ok := query.creation()
	query.close()
	if !ok {
		return false
	}
	if created != recorded {
		return true
	}
	waitable, err := processes.open(pid, windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE)
	if err != nil {
		return false
	}
	defer waitable.close()
	created, ok = waitable.creation()
	if !ok {
		return false
	}
	if created != recorded {
		return true
	}
	ended, err := waitable.ended()
	return err == nil && ended
}

// liveProcesses is the process table itself.
type liveProcesses struct{}

// present walks a process snapshot.
func (liveProcesses) present(pid uint32) (bool, bool) {
	return inSnapshot(pid)
}

// open calls OpenProcess.
func (liveProcesses) open(pid uint32, access uint32) (processHandle, error) {
	handle, err := windows.OpenProcess(access, false, pid)
	if err != nil {
		return nil, err
	}
	return liveProcess(handle), nil
}

// liveProcess is a handle OpenProcess answered.
type liveProcess windows.Handle

// creation reads the creation time through the handle.
func (p liveProcess) creation() (uint64, bool) {
	return creationTime(windows.Handle(p))
}

// ended asks WaitForSingleObject without waiting.
func (p liveProcess) ended() (bool, error) {
	event, err := windows.WaitForSingleObject(windows.Handle(p), 0)
	if err != nil {
		return false, err
	}
	return event == windows.WAIT_OBJECT_0, nil
}

// close closes the handle.
func (p liveProcess) close() {
	windows.CloseHandle(windows.Handle(p))
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
