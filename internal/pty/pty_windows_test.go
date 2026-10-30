//go:build windows

package pty

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"golang.org/x/sys/windows"

	"studio/internal/procjob"
)

// The test process gives up its standard handles, as Dinah's window does
// (ReleaseStandardHandles); go test's own output does not use them, since
// os.Stdout keeps the handle it was given at start.
func TestMain(m *testing.M) {
	if err := ReleaseStandardHandles(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func cmdExe(t *testing.T) string {
	t.Helper()
	p := filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
	if _, err := os.Stat(p); err != nil {
		t.Skip("cmd.exe is not available")
	}
	return p
}

// run starts cmd in a pseudoconsole, writes input, and returns everything
// drawn and the exit code once the program ends and the console is closed.
func run(t *testing.T, cmd Command, input string) (string, int) {
	t.Helper()
	s, err := Start(cmd, 100, 30)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	read := make(chan struct{})
	go func() { io.Copy(&out, s); close(read) }()
	if input != "" {
		if _, err := s.Write([]byte(input)); err != nil {
			t.Fatal(err)
		}
	}
	codes := make(chan int, 1)
	go func() { code, _ := s.Wait(); codes <- code }()
	var code int
	select {
	case code = <-codes:
	case <-time.After(20 * time.Second):
		s.Close()
		t.Fatalf("the program did not end; it drew %q", out.String())
	}
	s.Close()
	select {
	case <-read:
	case <-time.After(10 * time.Second):
		t.Fatal("the output did not end after the console closed")
	}
	return out.String(), code
}

func TestAProgramDrawsInThePseudoconsole(t *testing.T) {
	exe := cmdExe(t)
	out, code := run(t, Command{Path: exe, Args: []string{"cmd.exe", "/d", "/c", "echo hello from the pseudoconsole& exit 3"},
		Env: append(os.Environ(), "DINAH_PTY_TEST=1")}, "")
	if !strings.Contains(out, "hello from the pseudoconsole") || code != 3 {
		t.Fatalf("drew %q and ended with %d", out, code)
	}
}

func TestTypedTextReachesTheProgram(t *testing.T) {
	exe := cmdExe(t)
	dir := t.TempDir()
	out, code := run(t, Command{Path: exe, Args: []string{"cmd.exe", "/d", "/q", "/k", "prompt $g"}, Dir: dir,
		Env: append(os.Environ(), "DINAH_PTY_GREETING=typed text arrived")}, "echo %DINAH_PTY_GREETING% in %CD%\r\nexit 0\r\n")
	if !strings.Contains(out, "typed text arrived in "+dir) || code != 0 {
		t.Fatalf("drew %q and ended with %d", out, code)
	}
}

func TestTheConsoleCanBeResizedAndClosedEarly(t *testing.T) {
	exe := cmdExe(t)
	s, err := Start(Command{Path: exe, Args: []string{"cmd.exe", "/d", "/q", "/k"}}, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	go io.Copy(io.Discard, s)
	if err := s.Resize(120, 40); err != nil {
		t.Fatal(err)
	}
	if err := s.Resize(0, 40); err == nil {
		t.Fatal("a terminal with no columns was accepted")
	}
	done := make(chan struct{})
	go func() { s.Wait(); close(done) }()
	s.Close()
	select {
	case <-done:
	case <-time.After(CloseGrace + 5*time.Second):
		t.Fatal("closing the console did not end the shell")
	}
	if err := s.Resize(80, 24); err == nil {
		t.Fatal("a closed console was resized")
	}
}

// detachedChildEnv makes this test program, started by a shell in a
// pseudoconsole, leave its console with FreeConsole and sleep. A process
// with no console receives no CTRL_CLOSE_EVENT, as a windowed program does
// not, so it stands in for one without showing a window. It writes its
// process ID to the file the variable gives once it has left the console.
const detachedChildEnv = "DINAH_PTY_TEST_DETACHED_CHILD"

func init() {
	pidFile := os.Getenv(detachedChildEnv)
	if pidFile == "" {
		return
	}
	if r, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("FreeConsole").Call(); r == 0 {
		os.Exit(2)
	}
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		os.Exit(2)
	}
	time.Sleep(10 * time.Minute)
	os.Exit(0)
}

// startShell starts a Command Prompt in a pseudoconsole, reads what it
// draws, and closes it when the test ends.
func startShell(t *testing.T) *Session {
	t.Helper()
	s, err := Start(Command{Path: cmdExe(t), Args: []string{"cmd.exe", "/d", "/q", "/k"}}, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	go io.Copy(io.Discard, s)
	t.Cleanup(func() { s.Close() })
	return s
}

// openProcess opens a process for the checks these tests make, and closes
// it when the test ends.
func openProcess(t *testing.T, pid int) windows.Handle {
	t.Helper()
	p, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { windows.CloseHandle(p) })
	return p
}

// startDetachedChild has the shell start this test program as a detached
// child and returns it. The test ends the child when it is done.
func startDetachedChild(t *testing.T, s *Session) windows.Handle {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	if _, err := s.Write([]byte("set \"" + detachedChildEnv + "=" + pidFile + "\" && \"" + exe + "\"\r\n")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		if b, err := os.ReadFile(pidFile); err == nil && len(b) > 0 {
			if pid, err := strconv.Atoi(string(b)); err == nil {
				child := openProcess(t, pid)
				t.Cleanup(func() { windows.TerminateProcess(child, 0) })
				return child
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("the shell did not start the child")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func inJob(t *testing.T, process, job windows.Handle) bool {
	t.Helper()
	member, err := procjob.IsProcessInJob(process, job)
	if err != nil {
		t.Fatal(err)
	}
	return member
}

func inAppJob(t *testing.T, process windows.Handle) bool {
	t.Helper()
	member, err := procjob.InAppJob(process)
	if err != nil {
		t.Fatal(err)
	}
	return member
}

func ended(process windows.Handle, within time.Duration) bool {
	ev, err := windows.WaitForSingleObject(process, uint32(within/time.Millisecond))
	return err == nil && ev == windows.WAIT_OBJECT_0
}

// The shell runs in a job of its own outside the app's lifetime job, and a
// program it starts is in neither, so it can outlive Dinah.
func TestTheShellIsInAJobOfItsOwnAndItsChildInNeither(t *testing.T) {
	s := startShell(t)
	if !s.ChildrenOutsideAppJob() || s.job == 0 {
		t.Fatal("the shell was not started in a job of its own")
	}
	shell := openProcess(t, s.Pid())
	if !inJob(t, shell, s.job) {
		t.Fatal("the shell is not in its own job")
	}
	if inAppJob(t, shell) {
		t.Fatal("the shell is still in the app's lifetime job")
	}
	child := startDetachedChild(t, s)
	if inJob(t, child, s.job) {
		t.Fatal("the shell's child is in the shell's job")
	}
	if inAppJob(t, child) {
		t.Fatal("the shell's child is in the app's lifetime job")
	}
}

// Closing the shell's job, as happens when Dinah exits in any way, ends the
// shell and leaves its detached child running.
func TestClosingTheShellsJobEndsTheShellAndNotItsDetachedChild(t *testing.T) {
	s := startShell(t)
	shell := openProcess(t, s.Pid())
	child := startDetachedChild(t, s)
	s.closeJob()
	if !ended(shell, 10*time.Second) {
		t.Fatal("closing the shell's job did not end the shell")
	}
	if ended(child, 2*time.Second) {
		t.Fatal("the detached child ended with the shell's job")
	}
}

// Closing the session leaves the shell's detached child running.
func TestClosingTheConsoleLeavesADetachedChildRunning(t *testing.T) {
	s := startShell(t)
	child := startDetachedChild(t, s)
	s.Close()
	if _, err := s.Wait(); err != nil {
		t.Fatal(err)
	}
	if ended(child, 2*time.Second) {
		t.Fatal("the detached child ended when the console closed")
	}
}

// When the shell cannot be started outside the app's lifetime job, it is
// started inside it, in no job of its own, and closing still ends it.
func TestAShellThatCannotBreakAwayRunsInTheAppsJob(t *testing.T) {
	noBreakaway = true
	defer func() { noBreakaway = false }()
	s := startShell(t)
	if s.ChildrenOutsideAppJob() || s.job != 0 {
		t.Fatal("the shell reports a job of its own")
	}
	shell := openProcess(t, s.Pid())
	if !inAppJob(t, shell) {
		t.Fatal("the shell is not in the app's lifetime job")
	}
	child := startDetachedChild(t, s)
	if !inAppJob(t, child) {
		t.Fatal("the shell's child is not in the app's lifetime job")
	}
	s.Close()
	if !ended(shell, CloseGrace+5*time.Second) {
		t.Fatal("closing the console did not end the shell")
	}
}

func TestTheEnvironmentBlockIsSortedWithLastValuesKept(t *testing.T) {
	block, err := environmentUTF16([]string{"b=2", "A=1", "B=3", "Path=x"})
	if err != nil {
		t.Fatal(err)
	}
	want := "A=1\x00B=3\x00Path=x\x00\x00"
	if got := string(utf16.Decode(block)); got != want {
		t.Fatalf("block %q, want %q", got, want)
	}
	if _, err := environmentUTF16([]string{"novalue"}); err == nil {
		t.Fatal("an entry without = was accepted")
	}
}
