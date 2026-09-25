package screen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// recordingConsole stands in for a console screen buffer: it keeps the cursor
// and the attributes the way the documented functions change them, advances
// the cursor one cell per rune written the way WriteConsole's page says the
// cursor "advances as characters are written", and records every call.
type recordingConsole struct {
	info          BufferInfo
	cursorSize    uint32
	cursorVisible bool
	calls         []string
	text          []string
	fills         [][4]int
}

// newRecordingConsole is a window of width by height whose top is buffer row
// top, holding the attributes a console starts with.
func newRecordingConsole(width, height, top int) *recordingConsole {
	return &recordingConsole{
		info:          BufferInfo{Attributes: 0x0017, Left: 0, Top: top, Right: width - 1, Bottom: top + height - 1},
		cursorSize:    25,
		cursorVisible: true,
	}
}

func (r *recordingConsole) ScreenBufferInfo() (BufferInfo, error) {
	r.calls = append(r.calls, "GetConsoleScreenBufferInfo")
	return r.info, nil
}

func (r *recordingConsole) SetCursorPosition(x, y int) error {
	r.calls = append(r.calls, fmt.Sprintf("SetConsoleCursorPosition %d,%d", x, y))
	r.info.CursorX, r.info.CursorY = x, y
	return nil
}

func (r *recordingConsole) SetTextAttribute(attributes uint16) error {
	r.calls = append(r.calls, fmt.Sprintf("SetConsoleTextAttribute %#04x", attributes))
	r.info.Attributes = attributes
	return nil
}

func (r *recordingConsole) FillCharacter(character rune, length, x, y int) error {
	r.calls = append(r.calls, fmt.Sprintf("FillConsoleOutputCharacterW %q %d at %d,%d", character, length, x, y))
	r.fills = append(r.fills, [4]int{x, y, length, 0})
	return nil
}

func (r *recordingConsole) FillAttribute(attributes uint16, length, x, y int) error {
	r.calls = append(r.calls, fmt.Sprintf("FillConsoleOutputAttribute %#04x %d at %d,%d", attributes, length, x, y))
	r.fills = append(r.fills, [4]int{x, y, length, 1})
	return nil
}

func (r *recordingConsole) CursorInfo() (uint32, bool, error) {
	r.calls = append(r.calls, "GetConsoleCursorInfo")
	return r.cursorSize, r.cursorVisible, nil
}

func (r *recordingConsole) SetCursorInfo(size uint32, visible bool) error {
	r.calls = append(r.calls, fmt.Sprintf("SetConsoleCursorInfo %d %v", size, visible))
	r.cursorSize, r.cursorVisible = size, visible
	return nil
}

// Write is the text half: what consolewriter would hand WriteConsoleW. It
// advances the cursor, and a line feed moves it to the start of the next row.
func (r *recordingConsole) Write(p []byte) (int, error) {
	text := string(p)
	r.text = append(r.text, text)
	r.calls = append(r.calls, fmt.Sprintf("WriteConsoleW %q", text))
	for _, c := range text {
		if c == '\n' {
			r.info.CursorX = 0
			r.info.CursorY++
			continue
		}
		r.info.CursorX++
	}
	return len(p), nil
}

// TestTheConsoleLayerWritesNoControlSequence is dinah-288/criteria/6: a
// coloured one-shot line and a watch frame through the console layer put no
// U+001B into any text, colour goes through SetConsoleTextAttribute,
// placement through SetConsoleCursorPosition, erasing through
// FillConsoleOutputCharacterW with a count ending at srWindow.Right, and
// after the frame and the restore the attributes and the cursor's
// visibility are what they were before. The window sits partway down the
// buffer, so a layer that addressed buffer rows rather than window rows
// would be caught.
func TestTheConsoleLayerWritesNoControlSequence(t *testing.T) {
	console := newRecordingConsole(40, 6, 100)
	scr := NewConsole(console, console)
	if !scr.Colour() {
		t.Fatal("the console layer reports it cannot colour")
	}
	if ok, reason, _ := scr.Live(); !ok {
		t.Fatalf("the console layer is not live: %s", reason)
	}
	line := []Segment{{Text: "○", Colour: None}, {Text: " 4  "}, {Text: "●", Colour: Blue}, {Text: " 5"}}
	if err := scr.Line(line); err != nil {
		t.Fatalf("line: %v", err)
	}
	if console.info.Attributes != 0x0017 {
		t.Errorf("after a one-shot line with a coloured segment the attributes are %#04x, want the 0x0017 the console started with", console.info.Attributes)
	}
	if err := scr.Begin(); err != nil {
		t.Fatalf("begin: %v", err)
	}
	if console.cursorVisible {
		t.Error("Begin left the cursor visible")
	}
	rows := [][]Segment{
		{{Text: "Board"}},
		{{Text: "✕", Colour: Red}, {Text: " 449"}},
		{{Text: "◆", Colour: Yellow}, {Text: " 12"}},
	}
	for i, row := range rows {
		if err := scr.Row(i+1, row); err != nil {
			t.Fatalf("row: %v", err)
		}
	}
	if err := scr.EraseBelow(len(rows)); err != nil {
		t.Fatalf("erase below: %v", err)
	}
	if err := scr.Row(6, []Segment{{Text: "watching since 09:40:00 · Ctrl+C stops"}}); err != nil {
		t.Fatalf("status row: %v", err)
	}
	if err := scr.Row(7, []Segment{{Text: "beyond the window"}}); err != nil {
		t.Fatalf("row beyond the window: %v", err)
	}
	if err := scr.Restore(6); err != nil {
		t.Fatalf("restore: %v", err)
	}

	for _, text := range console.text {
		if strings.ContainsRune(text, 0x1b) {
			t.Errorf("text reaching the console carries an escape: %q", text)
		}
		if strings.Contains(text, "beyond the window") {
			t.Error("a row beyond the window's bottom was written")
		}
	}
	for _, fill := range console.fills {
		x, y, length := fill[0], fill[1], fill[2]
		if x+length-1 != console.info.Right {
			t.Errorf("a fill at %d,%d of %d cells ends at %d, not at srWindow.Right %d", x, y, length, x+length-1, console.info.Right)
		}
		if y < console.info.Top || y > console.info.Bottom {
			t.Errorf("a fill at row %d is outside the window %d to %d", y, console.info.Top, console.info.Bottom)
		}
	}
	joined := strings.Join(console.calls, "\n")
	for _, want := range []string{
		"SetConsoleCursorPosition 0,100",
		"SetConsoleCursorPosition 0,102",
		"SetConsoleCursorPosition 0,105",
		"SetConsoleTextAttribute 0x0019",
		"SetConsoleTextAttribute 0x001c",
		"SetConsoleTextAttribute 0x001e",
		"FillConsoleOutputCharacterW ' ' 35 at 5,101",
		"FillConsoleOutputAttribute 0x0017 35 at 5,101",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("no call %q among:\n%s", want, joined)
		}
	}
	if console.info.Attributes != 0x0017 {
		t.Errorf("the attributes after the restore are %#04x, want the saved 0x0017", console.info.Attributes)
	}
	if !console.cursorVisible || console.cursorSize != 25 {
		t.Errorf("the cursor after the restore is size %d visible %v, want 25 visible", console.cursorSize, console.cursorVisible)
	}
	if console.info.CursorY != 106 || console.info.CursorX != 0 {
		t.Errorf("the restore left the cursor at %d,%d, want the start of the line below row 6", console.info.CursorX, console.info.CursorY)
	}
}

// TestTheLayerNeverChangesAConsoleMode asserts the rest of
// dinah-288/criteria/6: no source of this package names SetConsoleMode, so
// no build of it can call the function, and the Console interface offers no
// method standing for it.
func TestTheLayerNeverChangesAConsoleMode(t *testing.T) {
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	read := 0
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		read++
		if strings.Contains(string(data), "SetConsoleMode") {
			t.Errorf("%s names SetConsoleMode", source)
		}
	}
	if read < 5 {
		t.Errorf("read %d sources of this package, so the sweep proves little", read)
	}
}
