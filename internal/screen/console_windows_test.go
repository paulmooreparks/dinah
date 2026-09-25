//go:build windows

package screen

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"

	"dinah/internal/consolewriter"

	"golang.org/x/sys/windows"
)

// realConsoleChild marks the copy of this test binary that owns a console of
// its own and drives the real layer on it.
const realConsoleChild = "DINAH_SCREEN_REAL_CONSOLE_CHILD"

// realConsoleRan is what the child prints once it has driven the layer and
// read the console back, which is how the parent tells a run from a skip.
const realConsoleRan = "dinah-screen-real-console-ran"

// TestTheRealConsoleIsLeftAsItWasFound is dinah-288/criteria/32. It starts a
// copy of this test binary with CREATE_NEW_CONSOLE, which the page "Process
// Creation Flags" documents as giving the new process "a new console,
// instead of inheriting its parent's console", and with its standard streams
// on pipes back to this process. The copy opens CONOUT$, which the page
// "Console Handles" documents as a handle to the console's active screen
// buffer "even if STDIN and STDOUT have been redirected", drives the real
// layer through Begin, a frame and Restore, and reads the cursor's
// visibility and the text attributes back with GetConsoleCursorInfo and
// GetConsoleScreenBufferInfo.
//
// A new console is used rather than FreeConsole and AllocConsole in this
// process, because this process may be writing its own report to the
// console go test gave it, and detaching would lose that report. The test
// fails rather than skips when its precondition is missing: a copy that
// cannot open a console fails, and the parent fails unless the copy printed
// realConsoleRan, so a copy that skipped or ran nothing is caught.
func TestTheRealConsoleIsLeftAsItWasFound(t *testing.T) {
	if os.Getenv(realConsoleChild) != "" {
		driveTheRealConsole(t)
		return
	}
	var output bytes.Buffer
	child := exec.Command(os.Args[0], "-test.run=^TestTheRealConsoleIsLeftAsItWasFound$", "-test.count=1")
	child.Env = append(os.Environ(), realConsoleChild+"=1")
	child.Stdout = &output
	child.Stderr = &output
	child.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE, HideWindow: true}
	if err := child.Run(); err != nil {
		t.Fatalf("the copy owning a console failed: %v\n%s", err, output.String())
	}
	if !strings.Contains(output.String(), realConsoleRan) {
		t.Fatalf("the copy owning a console exited without driving the layer, so nothing was tested:\n%s", output.String())
	}
}

// driveTheRealConsole is the child's half.
func driveTheRealConsole(t *testing.T) {
	name, err := windows.UTF16PtrFromString("CONOUT$")
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(
		name,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		t.Fatalf("this copy was started with a new console and cannot open CONOUT$, so the precondition is missing: %v", err)
	}
	file := os.NewFile(uintptr(handle), "CONOUT$")
	defer file.Close()
	console := OpenConsole(handle)
	before, err := console.ScreenBufferInfo()
	if err != nil {
		t.Fatalf("GetConsoleScreenBufferInfo before Begin: %v", err)
	}
	_, visibleBefore, err := console.CursorInfo()
	if err != nil {
		t.Fatalf("GetConsoleCursorInfo before Begin: %v", err)
	}
	scr := Open(file, consolewriter.New(file))
	if !scr.Colour() {
		t.Fatal("Open on a real console did not choose the console layer")
	}
	if err := scr.Begin(); err != nil {
		t.Fatalf("begin: %v", err)
	}
	_, visibleDuring, err := console.CursorInfo()
	if err != nil {
		t.Fatalf("GetConsoleCursorInfo during the frame: %v", err)
	}
	if visibleDuring {
		t.Error("the cursor is visible during the frame")
	}
	frame := [][]Segment{
		{{Text: "Board"}},
		{{Text: "●", Colour: Blue}, {Text: " 598  claude  now"}},
		{{Text: "✕", Colour: Red}, {Text: " 449  operator-ruling"}},
		{{Text: "◆", Colour: Yellow}, {Text: " 12"}},
	}
	for i, row := range frame {
		if err := scr.Row(i+1, row); err != nil {
			t.Fatalf("row %d: %v", i+1, err)
		}
	}
	if err := scr.EraseBelow(len(frame)); err != nil {
		t.Fatalf("erase below: %v", err)
	}
	if err := scr.Restore(len(frame)); err != nil {
		t.Fatalf("restore: %v", err)
	}
	after, err := console.ScreenBufferInfo()
	if err != nil {
		t.Fatalf("GetConsoleScreenBufferInfo after Restore: %v", err)
	}
	_, visibleAfter, err := console.CursorInfo()
	if err != nil {
		t.Fatalf("GetConsoleCursorInfo after Restore: %v", err)
	}
	if after.Attributes != before.Attributes {
		t.Errorf("the text attributes after Restore are %#04x, want the %#04x saved before Begin", after.Attributes, before.Attributes)
	}
	if visibleAfter != visibleBefore {
		t.Errorf("the cursor's visibility after Restore is %v, want the %v saved before Begin", visibleAfter, visibleBefore)
	}
	os.Stdout.WriteString(realConsoleRan + "\n")
}
