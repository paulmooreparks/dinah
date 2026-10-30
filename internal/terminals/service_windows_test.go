//go:build windows

package terminals

import (
	"encoding/base64"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"studio/internal/pty"
	"sync"
	"testing"
	"time"
	"unsafe"
)

func TestMain(m *testing.M) {
	if err := pty.ReleaseStandardHandles(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// shellDirs are the folders the tests' shells start in, by test. A test's folder
// is made before its service, so that the service's cleanup, which closes the
// shells and waits for each console to be gone, runs before the folder is
// removed (cleanups run last in, first out).
var shellDirs sync.Map

func shellDir(t *testing.T) string {
	t.Helper()
	dir, _ := shellDirs.Load(t)
	return dir.(string)
}

func testService(t *testing.T) *Service {
	t.Helper()
	shellDirs.Store(t, t.TempDir())
	t.Cleanup(func() { shellDirs.Delete(t) })
	a := New("Dinah", func(string, any) {}, nil)
	t.Cleanup(a.CloseAll)
	return a
}

// Closing a Terminal straight after it starts ends its shell, even when the
// shell has not yet attached to its console and so never receives the close
// event: the shell is ended once pty.CloseGrace has passed. TerminalClose
// itself returns at once, and closing again does no harm.
func TestClosingATerminalEndsItsShell(t *testing.T) {
	a := testService(t)
	exited := make(chan struct{}, 1)
	a.emit = func(name string, data any) {
		if name == "terminal:exit" {
			exited <- struct{}{}
		}
	}
	started, err := a.StartShell("cmd", nil, shellDir(t), 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	shell := a.terminals[started.ID].session
	a.mu.Unlock()
	began := time.Now()
	if err := a.Close(started.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(started.ID); err != nil {
		t.Fatal(err)
	}
	if waited := time.Since(began); waited > time.Second {
		t.Fatalf("closing the Terminal kept the caller waiting %v", waited)
	}
	select {
	case <-exited:
	case <-time.After(pty.CloseGrace + 10*time.Second):
		// Which half failed decides where to look: a shell still running
		// is the close path, a shell that ended without terminal:exit is
		// the output pump. An investigation on 29 October 2026 could not
		// reproduce this failure and found the test waits, by design, the
		// whole grace when cmd has not attached to its console before the
		// close (docs/plans/cleanup.md, phase 0).
		ended := make(chan int, 1)
		go func() { code, _ := shell.Wait(); ended <- code }()
		select {
		case code := <-ended:
			t.Fatalf("closing the Terminal ended its shell (exit code %#x) but terminal:exit was not reported", code)
		case <-time.After(time.Second):
			t.Fatal("closing the Terminal did not end its shell")
		}
	}
}

// detachedChildEnv makes this test program, started by a shell in a
// Terminal, leave its console with FreeConsole and sleep. A process with no
// console receives no CTRL_CLOSE_EVENT when the pseudoconsole closes, as a
// windowed program started from the shell does not, so it stands in for one
// without showing a window. Its process ID is written to the file the
// variable gives, once it has left the console.
const detachedChildEnv = "DINAH_TEST_DETACHED_CHILD"

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

// startClosableTerminal starts a Command Prompt in a Terminal and returns its
// ID and a channel that receives once its end is reported.
func startClosableTerminal(t *testing.T, a *Service) (string, chan struct{}) {
	t.Helper()
	exited := make(chan struct{}, 1)
	a.emit = func(name string, data any) {
		if name == "terminal:exit" {
			exited <- struct{}{}
		}
	}
	started, err := a.StartShell("cmd", nil, shellDir(t), 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close(started.ID) })
	return started.ID, exited
}

// childOf waits for a process with the given executable name whose parent
// is the given process, and opens it with the access given.
func childOf(t *testing.T, parent int, exe string, access uint32) windows.Handle {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
		if err != nil {
			t.Fatal(err)
		}
		entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
		for err = windows.Process32First(snap, &entry); err == nil; err = windows.Process32Next(snap, &entry) {
			if int(entry.ParentProcessID) == parent && strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), exe) {
				windows.CloseHandle(snap)
				p, err := windows.OpenProcess(access, false, entry.ProcessID)
				if err != nil {
					t.Fatal(err)
				}
				return p
			}
		}
		windows.CloseHandle(snap)
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the shell did not start %s", exe)
	return 0
}

// Closing a Terminal ends a console program its shell is running, such as a
// dev server or ping -t, because the program is attached to the console
// and receives the close event.
func TestClosingATerminalEndsTheConsoleProgramsItsShellStarted(t *testing.T) {
	a := testService(t)
	id, exited := startClosableTerminal(t, a)
	shell, err := a.terminal(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Write(id, "ping -n 100 127.0.0.1\r\n"); err != nil {
		t.Fatal(err)
	}
	ping := childOf(t, shell.session.Pid(), "PING.EXE", windows.SYNCHRONIZE|windows.PROCESS_TERMINATE)
	defer windows.CloseHandle(ping)
	if err := a.Close(id); err != nil {
		t.Fatal(err)
	}
	limit := pty.CloseGrace + 10*time.Second
	select {
	case <-exited:
	case <-time.After(limit):
		t.Fatal("closing the Terminal did not end its shell")
	}
	if ev, err := windows.WaitForSingleObject(ping, uint32(limit/time.Millisecond)); err != nil || ev != windows.WAIT_OBJECT_0 {
		windows.TerminateProcess(ping, 1)
		t.Fatalf("ping was still running after the Terminal closed (%d, %v)", ev, err)
	}
}

// Closing a Terminal leaves running a program its shell started that is not
// attached to the console, as a windowed program such as Notepad is not.
// The test ends the program itself afterwards.
func TestClosingATerminalLeavesADetachedProgramRunning(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	a := testService(t)
	id, exited := startClosableTerminal(t, a)
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	if err := a.Write(id, "set \""+detachedChildEnv+"="+pidFile+"\" && \""+exe+"\"\r\n"); err != nil {
		t.Fatal(err)
	}
	var pid int
	deadline := time.Now().Add(20 * time.Second)
	for pid == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the shell did not start the child")
		}
		if b, err := os.ReadFile(pidFile); err == nil && len(b) > 0 {
			pid, _ = strconv.Atoi(string(b))
		}
		time.Sleep(20 * time.Millisecond)
	}
	child, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(child)
	defer windows.TerminateProcess(child, 0)
	if err := a.Close(id); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
	case <-time.After(pty.CloseGrace + 10*time.Second):
		t.Fatal("closing the Terminal did not end its shell")
	}
	if ev, err := windows.WaitForSingleObject(child, 2000); err != nil || ev != uint32(windows.WAIT_TIMEOUT) {
		t.Fatalf("the detached child ended when the Terminal closed (%d, %v)", ev, err)
	}
}

func TestCtrlCInterruptsNodeServerInPowerShell(t *testing.T) {
	for _, name := range []string{"pwsh.exe", "node.exe"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("%s is not installed", name)
		}
	}
	// The test process ignores Ctrl+C first, as one started with
	// CREATE_NEW_PROCESS_GROUP does, Dinah's window after Restart to Update
	// among them. A shell inheriting that would never see the Ctrl+C below
	// (https://learn.microsoft.com/windows/console/setconsolectrlhandler).
	ctrlHandler := windows.NewLazySystemDLL("kernel32.dll").NewProc("SetConsoleCtrlHandler")
	if r, _, err := ctrlHandler.Call(0, 1); r == 0 {
		t.Fatal(err)
	}
	t.Cleanup(func() { ctrlHandler.Call(0, 0) })
	a := testService(t)
	var mu sync.Mutex
	var drawn strings.Builder
	a.emit = func(name string, data any) {
		if name == "terminal:output" {
			b, _ := base64.StdEncoding.DecodeString(data.(map[string]string)["data"])
			mu.Lock()
			drawn.Write(b)
			mu.Unlock()
		}
	}
	started, err := a.StartShell("pwsh", nil, shellDir(t), 160, 40)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close(started.ID) })
	waitOutput := func(marker string, count int) {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			mu.Lock()
			found := strings.Count(drawn.String(), marker) >= count
			mu.Unlock()
			if found {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		mu.Lock()
		defer mu.Unlock()
		t.Fatalf("PowerShell did not produce %s; it drew %q", marker, drawn.String())
	}
	if err := a.Write(started.ID, "function prompt { 'interrupt-' + 'prompt> ' }\r"); err != nil {
		t.Fatal(err)
	}
	waitOutput("interrupt-prompt> ", 1)
	if err := a.Write(started.ID, "node -e \"require('http').createServer((q,s)=>s.end('ok')).listen(0,()=>console.log('interrupt-'+'started'))\"\r"); err != nil {
		t.Fatal(err)
	}
	waitOutput("interrupt-started", 1)
	if err := a.Write(started.ID, "\x03"); err != nil {
		t.Fatal(err)
	}
	waitOutput("interrupt-prompt> ", 2)
	if err := a.Write(started.ID, "Write-Output ('interrupt-' + 'recovered')\r"); err != nil {
		t.Fatal(err)
	}
	waitOutput("interrupt-recovered", 1)
}

func TestCommandUsesSharedPumpAndCallbacksCanReadCount(t *testing.T) {
	a := testService(t)
	exited := make(chan int, 1)
	a.emit = func(name string, data any) {
		_ = a.Count()
		if name == "terminal:exit" {
			exited <- data.(map[string]any)["code"].(int)
		}
	}
	folder := t.TempDir()
	started, err := a.StartCommand("cmd.exe", []string{"/d", "/c", "exit 7"}, folder, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	if started.Shell != "command" || started.Name != "cmd.exe" || started.Folder != folder {
		t.Fatalf("command metadata: %+v", started)
	}
	select {
	case code := <-exited:
		if code != 7 {
			t.Fatalf("command exit: %d", code)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("command pump or callback did not finish")
	}
	if a.Count() != 0 {
		t.Fatal("ended command remained registered")
	}
}

// Once a Terminal has been closed, nothing of it holds its folder: Close
// returns when the program has ended and the console has finished closing, so
// the folder can be removed straight away.
func TestAClosedTerminalLeavesItsFolderFree(t *testing.T) {
	a := New("Dinah", func(string, any) {}, nil)
	dir, err := os.MkdirTemp("", "terminal-folder-")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.StartShell("cmd", nil, dir, 80, 24); err != nil {
		t.Fatal(err)
	}
	a.CloseAll()
	if err := os.Remove(dir); err != nil {
		t.Fatalf("the closed Terminal's folder could not be removed: %v", err)
	}
}
