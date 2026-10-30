//go:build windows

// Package procjob ties the processes Dinah starts to Dinah's own life, so
// none outlives it: the agents and the extensions. A Terminal's shell is
// started outside this job, in a job of its own that also ends with Dinah,
// so that what the shell starts is in neither (see internal/pty).
package procjob

import (
	"errors"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The app's job object, created once. A job object with
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE ends every process in it when the last
// handle to the job closes, which happens when the app exits in any way,
// including a crash (https://learn.microsoft.com/windows/win32/api/winnt/ns-winnt-jobobject_basic_limit_information).
// A process started by one in the job is in the job too, unless it is started
// with CREATE_BREAKAWAY_FROM_JOB, which JOB_OBJECT_LIMIT_BREAKAWAY_OK permits.
// Dinah joins its own job once an extension host starts, and the new build
// it starts on Restart to Update must break away, or it ends with the old one.
var (
	once sync.Once
	job  windows.Handle
)

func appJob() windows.Handle {
	once.Do(func() {
		h, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK
		if _, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
			windows.CloseHandle(h)
			return
		}
		job = h
	})
	return job
}

// Tie puts a started process in the app's job object. A process that cannot
// be put there runs on untied, as it would without the job.
func Tie(pid int) {
	j := appJob()
	if j == 0 || pid <= 0 {
		return
	}
	p, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(p)
	_ = windows.AssignProcessToJobObject(j, p)
}

// TieRequired refuses launch when the process cannot join the lifetime job.
// Extensions put Dinah's own process in the job before they start, so that
// what they start is in the job from its creation, and so does the Terminal,
// whose shell leaves the job only when it can be started with
// CREATE_BREAKAWAY_FROM_JOB: "By default, all child processes are associated with the
// immediate job and every job in the parent job chain"
// (https://learn.microsoft.com/windows/win32/api/jobapi2/nf-jobapi2-assignprocesstojobobject).
func TieRequired(pid int) error {
	j := appJob()
	if j == 0 {
		return errors.New("could not create the process lifetime job")
	}
	p, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(p)
	if member, err := IsProcessInJob(p, j); err == nil && member {
		return nil
	}
	return windows.AssignProcessToJobObject(j, p)
}

// isProcessInJob is called directly, because golang.org/x/sys does not wrap
// it.
var isProcessInJob = windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessInJob")

// IsProcessInJob reports whether the process is in the job, by
// IsProcessInJob (https://learn.microsoft.com/windows/win32/api/jobapi/nf-jobapi-isprocessinjob).
// A process in a nested job is in every job of its parent job chain too
// (https://learn.microsoft.com/windows/win32/procthread/nested-jobs).
func IsProcessInJob(process, job windows.Handle) (bool, error) {
	var member int32
	if ok, _, err := isProcessInJob.Call(uintptr(process), uintptr(job), uintptr(unsafe.Pointer(&member))); ok == 0 {
		return false, err
	}
	return member != 0, nil
}

// InAppJob reports whether the process is in the app's job.
func InAppJob(process windows.Handle) (bool, error) {
	j := appJob()
	if j == 0 {
		return false, errors.New("could not create the process lifetime job")
	}
	return IsProcessInJob(process, j)
}
