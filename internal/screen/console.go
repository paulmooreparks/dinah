package screen

import (
	"errors"
	"io"

	"dinah/internal/contract"
)

// The reason a screen that is not a terminal gives, and the error its
// drawing methods return. Neither is ever reached through the watch, which
// asks Live first and refuses before it draws.
const reasonNotATerminal = contract.WatchNotATerminal

var errNotATerminal = errors.New("screen: output is not a terminal")

// BufferInfo is what GetConsoleScreenBufferInfo reports that this layer
// reads: the cursor, the attributes text is written in, and srWindow, the
// buffer coordinates of the window's upper-left and lower-right corners. All
// of them are zero-based buffer coordinates.
type BufferInfo struct {
	// CursorX and CursorY are dwCursorPosition.
	CursorX, CursorY int
	// Attributes is wAttributes, the attributes text is written in.
	Attributes uint16
	// Left, Top, Right and Bottom are srWindow, inclusive at both ends.
	Left, Top, Right, Bottom int
}

// Console is the classic console functions this layer calls, one method per
// function. The Windows build binds each to the function of the same name in
// kernel32, and a test supplies its own to record every call.
type Console interface {
	// ScreenBufferInfo is GetConsoleScreenBufferInfo.
	ScreenBufferInfo() (BufferInfo, error)
	// SetCursorPosition is SetConsoleCursorPosition.
	SetCursorPosition(x, y int) error
	// SetTextAttribute is SetConsoleTextAttribute.
	SetTextAttribute(attributes uint16) error
	// FillCharacter is FillConsoleOutputCharacterW.
	FillCharacter(character rune, length, x, y int) error
	// FillAttribute is FillConsoleOutputAttribute.
	FillAttribute(attributes uint16, length, x, y int) error
	// CursorInfo is GetConsoleCursorInfo, reporting dwSize and bVisible.
	CursorInfo() (size uint32, visible bool, err error)
	// SetCursorInfo is SetConsoleCursorInfo.
	SetCursorInfo(size uint32, visible bool) error
}

// The character attribute bits the console layer writes, as the page
// "Console Screen Buffers" documents them under "Character attributes".
const (
	foregroundBlue      = 0x0001
	foregroundGreen     = 0x0002
	foregroundRed       = 0x0004
	foregroundIntensity = 0x0008
	backgroundBits      = 0x0010 | 0x0020 | 0x0040 | 0x0080
)

// NewConsole returns the console layer over a set of console functions,
// writing text through text. The Windows build opens it on the real
// functions; a test opens it on a recorder.
func NewConsole(console Console, text io.Writer) Screen {
	return &consoleScreen{console: console, text: text}
}

// consoleScreen draws with the classic console functions and writes no
// control sequence. Every placement is absolute, in buffer coordinates read
// from srWindow at the moment of the call, so a window scrolled or resized
// between two rows still gets each row where the window now is.
type consoleScreen struct {
	console Console
	text    io.Writer
	// saved says Attributes, CursorSize and CursorVisible hold what the
	// console carried before this layer changed anything.
	saved         bool
	attributes    uint16
	cursorSize    uint32
	cursorVisible bool
}

func (c *consoleScreen) Colour() bool { return true }

// Live is true on every console, because each function this layer calls acts
// on any console screen buffer and none of them depends on a console mode.
func (c *consoleScreen) Live() (bool, string, string) { return true, "", "" }

// save records the attributes and the cursor before the first change, once.
func (c *consoleScreen) save() error {
	if c.saved {
		return nil
	}
	info, err := c.console.ScreenBufferInfo()
	if err != nil {
		return err
	}
	size, visible, err := c.console.CursorInfo()
	if err != nil {
		return err
	}
	c.attributes = info.Attributes
	c.cursorSize = size
	c.cursorVisible = visible
	c.saved = true
	return nil
}

func (c *consoleScreen) Begin() error {
	if err := c.save(); err != nil {
		return err
	}
	if err := c.console.SetCursorInfo(c.cursorSize, false); err != nil {
		return err
	}
	return c.Clear()
}

// Clear fills every row of the window with spaces in the saved attributes,
// one row at a time, so a buffer wider than its window keeps whatever lies
// outside the window.
func (c *consoleScreen) Clear() error {
	info, err := c.console.ScreenBufferInfo()
	if err != nil {
		return err
	}
	for y := info.Top; y <= info.Bottom; y++ {
		if err := c.eraseFrom(info, info.Left, y); err != nil {
			return err
		}
	}
	return nil
}

// Row writes one row. A row beyond the window's bottom is not written at
// all, because placing the cursor outside the window moves the window, which
// the SetConsoleCursorPosition page documents.
func (c *consoleScreen) Row(row int, segments []Segment) error {
	if err := c.save(); err != nil {
		return err
	}
	info, err := c.console.ScreenBufferInfo()
	if err != nil {
		return err
	}
	y := info.Top + row - 1
	if row < 1 || y > info.Bottom {
		return nil
	}
	if err := c.console.SetCursorPosition(info.Left, y); err != nil {
		return err
	}
	if err := c.write(segments); err != nil {
		return err
	}
	written, err := c.console.ScreenBufferInfo()
	if err != nil {
		return err
	}
	return c.eraseFrom(written, written.CursorX, written.CursorY)
}

func (c *consoleScreen) EraseBelow(row int) error {
	info, err := c.console.ScreenBufferInfo()
	if err != nil {
		return err
	}
	for y := info.Top + row; y <= info.Bottom; y++ {
		if err := c.eraseFrom(info, info.Left, y); err != nil {
			return err
		}
	}
	return nil
}

func (c *consoleScreen) Line(segments []Segment) error {
	if err := c.save(); err != nil {
		return err
	}
	if err := c.write(segments); err != nil {
		return err
	}
	_, err := io.WriteString(c.text, "\n")
	return err
}

func (c *consoleScreen) Reset() error {
	if !c.saved {
		return nil
	}
	return c.console.SetTextAttribute(c.attributes)
}

// Restore puts the attributes and the cursor back, then places the cursor at
// the start of the row named and ends that line, which leaves it on the line
// below whether or not the row was the window's last.
func (c *consoleScreen) Restore(afterRow int) error {
	if !c.saved {
		return nil
	}
	if err := c.console.SetTextAttribute(c.attributes); err != nil {
		return err
	}
	if err := c.console.SetCursorInfo(c.cursorSize, c.cursorVisible); err != nil {
		return err
	}
	info, err := c.console.ScreenBufferInfo()
	if err != nil {
		return err
	}
	y := info.Top + afterRow - 1
	if y > info.Bottom {
		y = info.Bottom
	}
	if y < info.Top {
		y = info.Top
	}
	if err := c.console.SetCursorPosition(info.Left, y); err != nil {
		return err
	}
	_, err = io.WriteString(c.text, "\n")
	return err
}

// write puts the segments at the cursor. A coloured segment is written
// between two calls to SetConsoleTextAttribute, which sets the attributes of
// characters written by WriteFile or WriteConsole after the call: one keeping
// the saved background and setting the colour, and one putting the saved
// attributes back. Each segment is one Write of a whole string, so no
// incomplete UTF-8 sequence is held across a console call.
func (c *consoleScreen) write(segments []Segment) error {
	for _, segment := range segments {
		if segment.Colour == None {
			if _, err := io.WriteString(c.text, segment.Text); err != nil {
				return err
			}
			continue
		}
		coloured := c.attributes&backgroundBits | consoleForeground(segment.Colour)
		if err := c.console.SetTextAttribute(coloured); err != nil {
			return err
		}
		if _, err := io.WriteString(c.text, segment.Text); err != nil {
			return err
		}
		if err := c.console.SetTextAttribute(c.attributes); err != nil {
			return err
		}
	}
	return nil
}

// eraseFrom fills a row with spaces in the saved attributes from x to the
// window's right edge, inclusive. The count ends at srWindow.Right, because
// the FillConsoleOutputCharacter page documents that a fill longer than the
// row continues on the next row. A position outside the window is not
// filled, since that page's own guidance reserves the region outside the
// window for the terminal's history.
func (c *consoleScreen) eraseFrom(info BufferInfo, x, y int) error {
	if y < info.Top || y > info.Bottom || x > info.Right {
		return nil
	}
	if x < info.Left {
		x = info.Left
	}
	length := info.Right - x + 1
	if err := c.console.FillCharacter(' ', length, x, y); err != nil {
		return err
	}
	attributes := c.attributes
	if !c.saved {
		attributes = info.Attributes
	}
	return c.console.FillAttribute(attributes, length, x, y)
}

// consoleForeground is the attribute bits of a colour: each is intensified,
// and yellow is red and green together, since the documented attributes
// carry no yellow of their own.
func consoleForeground(colour Colour) uint16 {
	switch colour {
	case Blue:
		return foregroundBlue | foregroundIntensity
	case Red:
		return foregroundRed | foregroundIntensity
	case Yellow:
		return foregroundRed | foregroundGreen | foregroundIntensity
	}
	return 0
}
