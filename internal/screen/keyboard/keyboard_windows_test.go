//go:build windows

package keyboard

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/windows"

	"dinah/internal/screen"
)

// recordedConsole is a console that records every call in order.
type recordedConsole struct {
	calls []string
	modes map[windows.Handle]uint32
	in    windows.Handle
}

func (c *recordedConsole) name(h windows.Handle) string {
	if h == c.in {
		return "in"
	}
	return "out"
}

func (c *recordedConsole) GetConsoleMode(h windows.Handle, mode *uint32) error {
	*mode = c.modes[h]
	c.calls = append(c.calls, "get "+c.name(h))
	return nil
}

func (c *recordedConsole) SetConsoleMode(h windows.Handle, mode uint32) error {
	c.modes[h] = mode
	c.calls = append(c.calls, "set "+c.name(h)+" "+strconv.FormatUint(uint64(mode), 16))
	return nil
}

func (c *recordedConsole) FlushConsoleInputBuffer(h windows.Handle) error {
	c.calls = append(c.calls, "flush "+c.name(h))
	return nil
}

func (c *recordedConsole) Write(out *os.File, text string) error {
	c.calls = append(c.calls, "write "+strconv.Quote(text))
	return nil
}

// TestTheConsoleEntryEnablesPasteOnlyAfterVirtualTerminalProcessing is the
// order half of dinah-603/criteria/36: the entry saves and flushes the input
// handle and sets it to ENABLE_WINDOW_INPUT alone, sets
// ENABLE_VIRTUAL_TERMINAL_PROCESSING and DISABLE_NEWLINE_AUTO_RETURN on the
// output handle, and only then writes ESC [?2004h; leaving writes ESC [?2004l
// before it restores the output mode it saved, and then restores the input
// mode.
func TestTheConsoleEntryEnablesPasteOnlyAfterVirtualTerminalProcessing(t *testing.T) {
	in, err := os.CreateTemp(t.TempDir(), "in")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	recorded := &recordedConsole{in: windows.Handle(in.Fd()), modes: map[windows.Handle]uint32{
		windows.Handle(in.Fd()):  0x1f7,
		windows.Handle(out.Fd()): 0x3,
	}}
	saved := consoleAPI
	consoleAPI = recorded
	defer func() { consoleAPI = saved }()
	keyboard, err := EnterKeyboard(in, out, nil)
	if err != nil {
		t.Fatalf("enter: %v", err)
	}
	if err := keyboard.Leave(); err != nil {
		t.Fatalf("leave: %v", err)
	}
	vt := strconv.FormatUint(uint64(0x3|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING|windows.DISABLE_NEWLINE_AUTO_RETURN), 16)
	want := []string{
		"get in",
		"flush in",
		"set in " + strconv.FormatUint(windows.ENABLE_WINDOW_INPUT, 16),
		"get out",
		"set out " + vt,
		"write " + strconv.Quote(screen.BracketedPasteOn),
		"write " + strconv.Quote(screen.BracketedPasteOff),
		"set out 3",
		"set in 1f7",
	}
	if strings.Join(recorded.calls, "\n") != strings.Join(want, "\n") {
		t.Errorf("the calls ran in this order:\n%s\nwanted:\n%s", strings.Join(recorded.calls, "\n"), strings.Join(want, "\n"))
	}
}
