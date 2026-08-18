package main

import (
	"os"
	"strconv"
	"strings"

	"dinah/internal/textwidth"

	"golang.org/x/term"
)

// cell is one field of a row together with the column it is padded to.
type cell struct {
	// text is what the field draws.
	text string
	// width is the column the field is padded to, counted in the columns a
	// terminal gives it and not in characters.
	width int
}

// row is one line of columnar output before layout: the indent it starts at,
// the cells that each own a column, and a tail that takes whatever is left of
// the line.
//
// Every columnar line the head prints is built as one of these and laid out by
// formatRow. A row laid out anywhere else drifts the moment a field carries
// text a rune count measures differently from a screen does, which is what
// TestNoRowIsLaidOutOutsideTheOneRenderer refuses.
type row struct {
	// indent is the display column the row's first cell starts at.
	indent int
	// cells are the fields that each own a declared column, in print order.
	cells []cell
	// tail takes whatever is left of the line and is padded to nothing.
	tail string
}

// minTailColumns is how much of a line a continuation always keeps for its own
// text. A row indented deeper than the window less this falls back, so a
// narrow window never receives an indent wider than the text it leaves room
// for.
const minTailColumns = 20

// displayWidth reports how many terminal columns text occupies. It takes one
// unit at a time, where a unit is a whole emoji sequence as UTS #51 defines
// one, a regional indicator pair, or a single rune.
//
// The measure itself lives in internal/textwidth rather than here, because the
// guard that reads the message catalogs for a hand-laid-out row needs the same
// answer and cannot import a package main. Two measures that can disagree
// would let the guard pass output the binary misaligns.
func displayWidth(text string) int {
	return textwidth.Columns(text)
}

// pad returns text widened to width. It is called only on a field that fits,
// since formatRow gives a field that reaches its column the rest of its line
// instead, so the branch returning text untouched is unreachable through the
// renderer and exists so that a direct caller gets its own text back rather
// than a negative repeat count.
func pad(text string, width int) string {
	drawn := displayWidth(text)
	if drawn >= width {
		return text
	}
	return text + strings.Repeat(" ", width-drawn)
}

// formatRow lays a row out for a window of the given width, in columns. A
// window of zero means the width is not known, which is the piped case, and
// the layout is then unbounded. The result may carry newlines.
//
// A cell under its column is padded in place. A cell that reaches its column
// takes the rest of its own line, and every field after it, cells and tail
// alike, resumes on a continuation line indented to where that cell's column
// would have ended. Each remaining cell is tested independently, so two
// overflowing cells in one row each get their own line. Whether a cell
// overflowed or not, the column of the cell after it is where the declared
// layout puts it.
//
// Nothing here truncates and nothing breaks a word. A field longer than the
// window is written whole and the terminal wraps it where it likes, which
// keeps a path copyable.
func formatRow(r row, window int) string {
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", r.indent))
	column := r.indent
	for _, c := range r.cells {
		if displayWidth(c.text) < c.width {
			b.WriteString(pad(c.text, c.width))
			column += c.width
			continue
		}
		b.WriteString(c.text)
		b.WriteString("\n")
		column += c.width
		b.WriteString(strings.Repeat(" ", continuation(column, r.indent, window)))
	}
	b.WriteString(r.tail)
	return b.String()
}

// continuation clamps a continuation line's indent so the line keeps
// minTailColumns for its own text, and never takes it below the row's own
// indent. An unknown window clamps nothing.
func continuation(wanted, indent, window int) int {
	if window == 0 {
		return wanted
	}
	floor := window - minTailColumns
	if floor < indent {
		floor = indent
	}
	if wanted > floor {
		return floor
	}
	return wanted
}

// windowWidth reports the columns the window gives, or zero when no documented
// source answers.
//
// COLUMNS decides whenever the environment carries it, which POSIX defines in
// XBD Chapter 8 as the user's preferred width in column positions for the
// terminal screen. A value that parses and is positive is the width, clamped
// up to minTailColumns since no layout helps below that; a value that is
// absent from the variable, empty, not a decimal integer, zero, or negative
// states nothing a layout can use, and the width is then unknown.
//
// The terminal is asked only when the environment carries no COLUMNS at all,
// through golang.org/x/term, which reads GetConsoleScreenBufferInfo on Windows
// and TIOCGWINSZ elsewhere. Neither PowerShell nor cmd.exe puts COLUMNS into a
// child's environment and a POSIX shell exports it only when asked, so the
// terminal query is what the interactive case actually runs on. It answers
// nothing when output is piped, which leaves the piped run unbounded and
// therefore identical in any terminal.
func windowWidth() int {
	if stated, ok := os.LookupEnv("COLUMNS"); ok {
		columns, err := strconv.Atoi(strings.TrimSpace(stated))
		if err != nil || columns <= 0 {
			return 0
		}
		return clampWindow(columns)
	}
	columns, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || columns <= 0 {
		return 0
	}
	return clampWindow(columns)
}

// clampWindow raises a stated width to the narrowest one a layout can use.
func clampWindow(columns int) int {
	if columns < minTailColumns {
		return minTailColumns
	}
	return columns
}

// rowLine renders a row at the width this session draws at.
func (s *session) rowLine(r row) string {
	return formatRow(r, s.width)
}

// row renders a row and writes it to stdout.
func (s *session) row(r row) {
	s.line(s.rowLine(r))
}
