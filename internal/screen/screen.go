// Package screen is the only code in the tree that changes a terminal's
// state. It colours text, places the cursor, erases rows and hides and shows
// the cursor, and it restores what it changed. Everything else the tool
// prints is plain text written through the session's own writer.
//
// Two layers implement it. On Windows the console layer calls the classic
// console functions and never writes a control sequence, so nothing
// consolewriter splits across two WriteConsoleW calls can ever be a sequence.
// Each of those functions is documented on its own page under
// https://learn.microsoft.com/en-us/windows/console/, and six of the seven
// this layer calls carry Microsoft's notice, quoted here as those pages carry
// it: "This document describes console platform functionality that is no
// longer a part of our ecosystem roadmap. We do not recommend using this
// content in new products, but we will continue to support existing usages
// for the indefinite future. Our preferred modern solution focuses on virtual
// terminal sequences for maximum compatibility in cross-platform scenarios."
// GetConsoleScreenBufferInfo carries no such notice. The classic functions
// were chosen anyway, on the operator's ruling of 2026-09-25, because every
// behaviour this layer relies on is documented on those pages, and nothing
// Microsoft documents says how a virtual-terminal sequence cut across two
// WriteConsole calls is parsed. The layer sits behind the Screen interface,
// so moving it to virtual-terminal sequences later changes no caller.
//
// Everywhere else the terminfo layer writes only strings the terminal's own
// compiled terminfo entry supplied, found by the TERM variable and read by
// the reader in this package, and it writes each row, its sequences included,
// in one Write.
//
// Both layers write text through the writer they were opened with, and the
// console layer interleaves those writes with console calls. That is correct
// only while every Write reaches the console before the call after it runs,
// which holds for the session's stdout because consolewriter.Writer does not
// buffer. A buffered writer placed between the session and this package
// would put text wherever the cursor stood when the buffer flushed rather
// than where the layer placed it.
package screen

import (
	"io"
	"os"
	"strings"
)

// Colour is a foreground colour a segment is drawn in. The four values are
// the only ones the board uses: status is always carried by a glyph as well,
// so colour only ever reinforces it.
type Colour int

const (
	// None draws a segment in whatever colour the terminal was already using.
	None Colour = iota
	// Blue marks a card somebody holds.
	Blue
	// Red marks a blocked card.
	Red
	// Yellow marks something waiting on the operator.
	Yellow
)

// Segment is one run of text in one colour. A line is a list of them, and the
// text of a line is the segments' texts joined, whatever their colours.
type Segment struct {
	// Text is what the segment draws. It carries no control character.
	Text string
	// Colour is what the segment is drawn in, None for the default.
	Colour Colour
}

// Screen is a terminal the tool may change. Rows are one-based and count from
// the top of the visible window.
type Screen interface {
	// Colour reports whether a foreground colour can be set on this terminal.
	Colour() bool
	// Live reports whether the terminal can be redrawn in place: cleared,
	// addressed a row at a time, and erased to the end of a row and below a
	// row. Where it cannot, reason is one of contract's Watch tokens and
	// extra names the missing capability for WatchMissingCapability. The
	// window's size is the caller's to check, since the caller reads it
	// through the same ladder every layout reads.
	Live() (ok bool, reason, extra string)
	// Begin saves what Restore puts back, hides the cursor where the
	// terminal can, and clears the window.
	Begin() error
	// Clear erases the whole window.
	Clear() error
	// Row places the cursor at the start of a row, writes the segments there
	// and erases the rest of the row.
	Row(row int, segments []Segment) error
	// EraseBelow erases every row after the one named.
	EraseBelow(row int) error
	// Line writes the segments at the cursor and ends the line, which is how
	// a drawing printed once reaches the terminal. It leaves the colour as it
	// found it.
	Line(segments []Segment) error
	// Reset puts the colour back to what it was before the first coloured
	// segment, which is what an interrupted drawing does before it stops.
	Reset() error
	// Restore puts back what Begin saved, shows the cursor, and leaves the
	// cursor at the start of the line below the row named, so that whatever
	// runs next writes beneath the last drawing.
	Restore(afterRow int) error
}

// Open opens the screen for a stream. text is the writer every segment's text
// goes through, which is the session's stdout writer, so text still reaches
// a Windows console through consolewriter. A stream that is not a terminal,
// including a nil one, gets a screen that changes nothing: its Colour is
// false, its Live answers not-a-terminal, and its Line writes the text alone.
func Open(f *os.File, text io.Writer) Screen {
	if f == nil || !isTerminal(f) {
		return Plain(text)
	}
	return openTerminal(f, text)
}

// Plain returns the screen of a stream that is not a terminal.
func Plain(text io.Writer) Screen {
	return plainScreen{text: text}
}

// plainScreen is a stream that is not a terminal, such as a file or a pipe.
// It never changes anything, because there is nothing to change.
type plainScreen struct {
	text io.Writer
}

func (plainScreen) Colour() bool { return false }

func (plainScreen) Live() (bool, string, string) { return false, reasonNotATerminal, "" }

func (plainScreen) Begin() error { return errNotATerminal }

func (plainScreen) Clear() error { return errNotATerminal }

func (plainScreen) Row(int, []Segment) error { return errNotATerminal }

func (plainScreen) EraseBelow(int) error { return errNotATerminal }

func (p plainScreen) Line(segments []Segment) error {
	_, err := io.WriteString(p.text, Text(segments)+"\n")
	return err
}

func (plainScreen) Reset() error { return nil }

func (plainScreen) Restore(int) error { return nil }

// Text is the text of a line: its segments joined, whatever their colours.
func Text(segments []Segment) string {
	var b strings.Builder
	for _, segment := range segments {
		b.WriteString(segment.Text)
	}
	return b.String()
}
