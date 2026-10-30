//go:build windows

package pty

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"

	"studio/internal/procjob"
)

// Session is a program running in a Windows pseudoconsole, made the way
// Microsoft documents it in "Creating a Pseudoconsole session"
// (https://learn.microsoft.com/windows/console/creating-a-pseudoconsole-session):
// two pipes, CreatePseudoConsole over their far ends, and the program
// started with the pseudoconsole as a process attribute.
type Session struct {
	console windows.Handle
	pid     int
	out     *os.File // what the program draws
	in      *os.File // what is typed to it

	exited  chan struct{} // closed once the program has ended
	code    int
	waitErr error

	closeOnce sync.Once
	mu        sync.Mutex
	closed    bool
	// process is the program's handle until watch releases it, after which
	// it is 0. It is read and released under mu, so Close never ends a
	// process through a handle that has been closed.
	process windows.Handle
	// job is the program's own job until it is closed, after which it is 0.
	// Only this process holds a handle to it. It is closed under mu.
	job windows.Handle
	// ownJob is true when the program was started outside the app's
	// lifetime job and in a job of its own; see start.
	ownJob bool
}

// noBreakaway makes start skip the attempt to start the program outside
// the app's lifetime job, as if that attempt had failed. Only tests set it.
var noBreakaway bool

// terminatedCode is the exit code of a program that did not end within
// CloseGrace and was ended by Close.
const terminatedCode = 1

// ReleaseStandardHandles sets this process's standard handles to none
// (SetStdHandle). A program in a pseudoconsole is started without
// STARTF_USESTDHANDLES, so by the STARTUPINFO documentation its standard
// input and output are the console's buffers, which are the
// pseudoconsole's. Whether a child takes its parent's own standard handles
// instead, when the parent has some, is not documented, so a process that
// runs pseudoconsoles gives its handles up first and that question does not
// arise. Dinah's window never uses them: it is a Windows GUI program, which
// writes its log to files.
func ReleaseStandardHandles() error {
	for _, h := range []uint32{windows.STD_INPUT_HANDLE, windows.STD_OUTPUT_HANDLE, windows.STD_ERROR_HANDLE} {
		if err := windows.SetStdHandle(h, 0); err != nil {
			return err
		}
	}
	return nil
}

// UpdateProcThreadAttribute is called directly, because the pseudoconsole
// attribute's value is the handle itself rather than a pointer to it, and
// golang.org/x/sys's wrapper takes the value as a pointer.
var updateProcThreadAttribute = windows.NewLazySystemDLL("kernel32.dll").NewProc("UpdateProcThreadAttribute")

// SetConsoleCtrlHandler is called directly, because golang.org/x/sys does
// not wrap it.
var setConsoleCtrlHandler = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetConsoleCtrlHandler")

// processCtrlC restores normal processing of Ctrl+C in this process, which
// the programs it starts inherit. SetConsoleCtrlHandler documents that a
// process ignoring Ctrl+C passes that on to its children, and that a null
// handler with FALSE restores normal processing
// (https://learn.microsoft.com/windows/console/setconsolectrlhandler). A
// process started with CREATE_NEW_PROCESS_GROUP begins with Ctrl+C disabled
// (https://learn.microsoft.com/windows/win32/procthread/process-creation-flags),
// as Dinah's window is after Restart to Update, and so may whatever started
// this process. A shell inheriting that would leave Ctrl+C typed in its
// Terminal without effect on the programs it runs, so a pseudoconsole's
// program is always started with Ctrl+C processed. The setting belongs to
// the whole process, which suits Dinah's window and the tests alike, since
// neither has reason to ignore Ctrl+C.
func processCtrlC() error {
	if r, _, err := setConsoleCtrlHandler.Call(0, 0); r == 0 {
		return fmt.Errorf("restore Ctrl+C for the terminal's program: %w", err)
	}
	return nil
}

// Start runs cmd in a new pseudoconsole of cols by rows cells.
func Start(cmd Command, cols, rows int) (*Session, error) {
	if cols < 1 || rows < 1 || cols > 32767 || rows > 32767 {
		return nil, fmt.Errorf("a terminal of %d by %d cells is not possible", cols, rows)
	}
	var inRead, inWrite, outRead, outWrite windows.Handle
	if err := windows.CreatePipe(&inRead, &inWrite, nil, 0); err != nil {
		return nil, fmt.Errorf("make the terminal's input pipe: %w", err)
	}
	if err := windows.CreatePipe(&outRead, &outWrite, nil, 0); err != nil {
		windows.CloseHandle(inRead)
		windows.CloseHandle(inWrite)
		return nil, fmt.Errorf("make the terminal's output pipe: %w", err)
	}
	// The pseudoconsole keeps its own copies of the far ends, which this
	// process closes once the program has started.
	defer windows.CloseHandle(inRead)
	defer windows.CloseHandle(outWrite)
	s := &Session{out: os.NewFile(uintptr(outRead), "terminal output"), in: os.NewFile(uintptr(inWrite), "terminal input"),
		exited: make(chan struct{})}
	if err := windows.CreatePseudoConsole(windows.Coord{X: int16(cols), Y: int16(rows)}, inRead, outWrite, 0, &s.console); err != nil {
		s.out.Close()
		s.in.Close()
		return nil, fmt.Errorf("make the pseudoconsole: %w", err)
	}
	if err := s.start(cmd); err != nil {
		windows.ClosePseudoConsole(s.console)
		s.out.Close()
		s.in.Close()
		return nil, err
	}
	return s, nil
}

// newJob makes the job object the program runs in.
//
// With JOB_OBJECT_LIMIT_SILENT_BREAKAWAY_OK, "child processes of any parent
// process associated with the job are not associated with the job"
// (https://learn.microsoft.com/windows/win32/procthread/job-objects), so
// what the program starts is in neither this job nor the app's, and a
// windowed program started from the shell outlives Dinah.
//
// With JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE, closing the last handle to the
// job "terminates all associated processes"
// (https://learn.microsoft.com/windows/win32/api/jobapi2/nf-jobapi2-createjobobjectw),
// which is the program alone. The job is made with no security attributes,
// so its handle "cannot be inherited" (the same page), and this process
// holds the only one. When this process ends, in any way, its handles are
// closed (https://learn.microsoft.com/windows/win32/procthread/terminating-a-process),
// so the program ends with Dinah, as it did inside the app's job.
func newJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, fmt.Errorf("make the terminal's job: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | windows.JOB_OBJECT_LIMIT_SILENT_BREAKAWAY_OK
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return 0, fmt.Errorf("set the terminal's job's limits: %w", err)
	}
	return job, nil
}

// start runs the program in the pseudoconsole, in a job of its own outside
// the app's lifetime job, so that programs it starts are outside every job
// of Dinah's and may outlive Dinah.
//
// Dinah's own process joins the app's lifetime job first
// (procjob.TieRequired). The program is created suspended
// (https://learn.microsoft.com/windows/win32/procthread/process-creation-flags)
// and with CREATE_BREAKAWAY_FROM_JOB, which the lifetime job's
// JOB_OBJECT_LIMIT_BREAKAWAY_OK allows: "the child process is not associated
// with the job"
// (https://learn.microsoft.com/windows/win32/api/winnt/ns-winnt-jobobject_basic_limit_information).
// While it is suspended it is assigned to a new job of its own (newJob),
// and only then resumed, so it starts nothing before it is in that job. The
// Job Objects page describes this way of putting a child in a new job:
// create it with CREATE_BREAKAWAY_FROM_JOB, then call
// AssignProcessToJobObject (https://learn.microsoft.com/windows/win32/procthread/job-objects).
//
// The pseudoconsole is given through the attribute list at creation, as
// before, and the Ctrl+C setting is inherited at creation too (processCtrlC),
// so neither depends on when the program is resumed.
//
// If the program cannot be created with CREATE_BREAKAWAY_FROM_JOB, which
// happens when this process is in a job that does not allow breakaway, it
// is created again without the flag and in no job of its own. It is then in
// the app's lifetime job from its creation, as every child of a process in a
// job is (https://learn.microsoft.com/windows/win32/api/jobapi2/nf-jobapi2-assignprocesstojobobject),
// and so is everything it starts, which ends when Dinah does.
// ChildrenOutsideAppJob reports which of the two happened.
func (s *Session) start(cmd Command) error {
	if err := procjob.TieRequired(os.Getpid()); err != nil {
		return fmt.Errorf("join the app's lifetime job: %w", err)
	}
	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return err
	}
	defer attrs.Delete()
	if r, _, err := updateProcThreadAttribute.Call(uintptr(unsafe.Pointer(attrs.List())), 0,
		windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, uintptr(s.console), unsafe.Sizeof(s.console), 0, 0); r == 0 {
		return fmt.Errorf("give the program its pseudoconsole: %w", err)
	}
	si := windows.StartupInfoEx{ProcThreadAttributeList: attrs.List()}
	si.StartupInfo.Cb = uint32(unsafe.Sizeof(si))
	line, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(cmd.Args))
	if err != nil {
		return err
	}
	app, err := windows.UTF16PtrFromString(cmd.Path)
	if err != nil {
		return err
	}
	var dir *uint16
	if cmd.Dir != "" {
		if dir, err = windows.UTF16PtrFromString(cmd.Dir); err != nil {
			return err
		}
	}
	env, err := environmentBlock(cmd.Env)
	if err != nil {
		return err
	}
	if err := processCtrlC(); err != nil {
		return err
	}
	job, err := newJob()
	if err != nil {
		return err
	}
	var pi windows.ProcessInformation
	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT | windows.CREATE_SUSPENDED)
	created := false
	if !noBreakaway {
		created = windows.CreateProcess(app, line, nil, nil, false, flags|windows.CREATE_BREAKAWAY_FROM_JOB, env, dir, &si.StartupInfo, &pi) == nil
	}
	if created {
		if err := windows.AssignProcessToJobObject(job, pi.Process); err != nil {
			// Outside the app's job and in no job of its own, the program
			// would outlive Dinah, so it is ended before it ever runs.
			windows.TerminateProcess(pi.Process, terminatedCode)
			windows.CloseHandle(pi.Thread)
			windows.CloseHandle(pi.Process)
			windows.CloseHandle(job)
			return fmt.Errorf("put %s in the terminal's job: %w", cmd.Path, err)
		}
		s.job, s.ownJob = job, true
	} else {
		windows.CloseHandle(job)
		if err := windows.CreateProcess(app, line, nil, nil, false, flags, env, dir, &si.StartupInfo, &pi); err != nil {
			return fmt.Errorf("start %s: %w", cmd.Path, err)
		}
	}
	if _, err := windows.ResumeThread(pi.Thread); err != nil {
		windows.TerminateProcess(pi.Process, terminatedCode)
		windows.CloseHandle(pi.Thread)
		windows.CloseHandle(pi.Process)
		s.closeJob()
		return fmt.Errorf("resume %s: %w", cmd.Path, err)
	}
	windows.CloseHandle(pi.Thread)
	s.process, s.pid = pi.Process, int(pi.ProcessId)
	go s.watch(pi.Process)
	return nil
}

// ChildrenOutsideAppJob reports whether the program runs in a job of its
// own outside the app's lifetime job, so that a program it starts is in no
// job of Dinah's and may outlive Dinah. It is false when the program had to
// be started inside the app's job; see start.
func (s *Session) ChildrenOutsideAppJob() bool { return s.ownJob }

// closeJob closes the program's job, if it has one and it is still open.
// While the program runs that ends it, by JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
// as Dinah's exit does; once it has ended the job is empty and closing it
// ends nothing.
func (s *Session) closeJob() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job != 0 {
		windows.CloseHandle(s.job)
		s.job = 0
	}
}

// watch waits for the program to end, keeps its exit code for Wait, and
// releases the process's handle and its job.
func (s *Session) watch(process windows.Handle) {
	defer close(s.exited)
	defer func() {
		s.mu.Lock()
		s.process = 0
		s.mu.Unlock()
		windows.CloseHandle(process)
		s.closeJob()
	}()
	s.code = -1
	if _, err := windows.WaitForSingleObject(process, windows.INFINITE); err != nil {
		s.waitErr = err
		return
	}
	var code uint32
	if err := windows.GetExitCodeProcess(process, &code); err != nil {
		s.waitErr = err
		return
	}
	s.code = int(code)
}

// environmentBlock makes the block CreateProcess takes: each NAME=value
// ended by a null, the whole ended by another, sorted by name without
// regard to case as "Changing Environment Variables" requires
// (https://learn.microsoft.com/windows/win32/procthread/changing-environment-variables).
// A name given twice keeps its last value.
func environmentBlock(env []string) (*uint16, error) {
	if len(env) == 0 {
		return nil, nil
	}
	block, err := environmentUTF16(env)
	if err != nil {
		return nil, err
	}
	return &block[0], nil
}

func environmentUTF16(env []string) ([]uint16, error) {
	byName := map[string]string{}
	var order []string
	for _, kv := range env {
		name, _, ok := strings.Cut(kv, "=")
		if !ok || strings.ContainsRune(kv, 0) {
			return nil, fmt.Errorf("the environment entry %q is not NAME=value", kv)
		}
		key := strings.ToUpper(name)
		if _, seen := byName[key]; !seen {
			order = append(order, key)
		}
		byName[key] = kv
	}
	sort.Strings(order)
	var block []uint16
	for _, key := range order {
		block = append(block, utf16.Encode([]rune(byName[key]))...)
		block = append(block, 0)
	}
	block = append(block, 0)
	return block, nil
}

// Read reads what the program drew. It returns an error once the
// pseudoconsole is closed and everything it wrote has been read, and the
// output pipe is released then.
func (s *Session) Read(p []byte) (int, error) {
	n, err := s.out.Read(p)
	if err != nil {
		s.out.Close()
	}
	return n, err
}

// Write sends typed text to the program.
func (s *Session) Write(p []byte) (int, error) { return s.in.Write(p) }

// Resize gives the pseudoconsole a new size in cells.
func (s *Session) Resize(cols, rows int) error {
	if cols < 1 || rows < 1 || cols > 32767 || rows > 32767 {
		return fmt.Errorf("a terminal of %d by %d cells is not possible", cols, rows)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("the terminal is closed")
	}
	return windows.ResizePseudoConsole(s.console, windows.Coord{X: int16(cols), Y: int16(rows)})
}

// Pid is the program's process ID.
func (s *Session) Pid() int { return s.pid }

// Wait waits for the program to end and returns its exit code. It may be
// called any number of times.
func (s *Session) Wait() (int, error) {
	<-s.exited
	return s.code, s.waitErr
}

// Close ends the program and the programs still attached to its console,
// and lets Read finish once the output is drained.
//
// It closes the pseudoconsole, which sends CTRL_CLOSE_EVENT to every program
// still connected to it
// (https://learn.microsoft.com/windows/console/closepseudoconsole). A
// program that does not exit on that event within the system's timeout is
// ended by the system
// (https://learn.microsoft.com/windows/console/handlerroutine). A program
// that has left the console, or was started without one, as a windowed
// program is, receives no event and goes on running.
//
// A program closed before it has attached to the console misses the event
// too, so if the program itself has not ended after CloseGrace, Close ends
// it with TerminateProcess. That ends that one process only: "Terminating a
// process does not cause child processes to be terminated"
// (https://learn.microsoft.com/windows/win32/api/processthreadsapi/nf-processthreadsapi-terminateprocess).
//
// Close returns once the program has ended or been ended and the console has
// been closed, so it may take CloseGrace and then until the output is read; callers that must not wait call it on a goroutine of their
// own. Further calls wait for the first to finish and then do nothing. The
// caller keeps reading until Read fails, because ClosePseudoConsole may wait
// for output still to be read.
func (s *Session) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		s.in.Close()
		// Before Windows 11 24H2, ClosePseudoConsole "will wait indefinitely
		// for the pseudoconsole to exit"
		// (https://learn.microsoft.com/windows/console/closepseudoconsole),
		// which a program that never attached may keep it from doing until
		// it is ended below. The grace is therefore counted from here, and
		// the console is closed on a goroutine of its own.
		grace := time.NewTimer(CloseGrace)
		defer grace.Stop()
		consoleGone := make(chan struct{})
		go func() {
			windows.ClosePseudoConsole(s.console)
			close(consoleGone)
		}()
		select {
		case <-s.exited:
		case <-grace.C:
			// TerminateProcess only begins the ending, so Close waits for the
			// process to be signaled, as the documentation advises. The handle
			// is used under mu, before watch can release it.
			s.mu.Lock()
			ending := s.process != 0 && windows.TerminateProcess(s.process, terminatedCode) == nil
			s.mu.Unlock()
			if ending {
				<-s.exited
			}
		}
		// ClosePseudoConsole returns once the console has been drained of its
		// output, which the caller goes on reading, so the console, its host
		// and the folders they hold are gone only when it has returned. The
		// wait is bounded, so that a console that is never drained cannot keep
		// a Close, and with it the program's exit, from returning.
		select {
		case <-consoleGone:
		case <-time.After(ConsoleCloseLimit):
		}
	})
	return nil
}
