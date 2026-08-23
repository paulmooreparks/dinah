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
	// wrapTail asks for the tail to be broken between words so that no line
	// reaches past the window, with each line after the first indented under
	// the column the tail began at. A row that says nothing has its tail
	// written whole, which is what every row drew before the arguments table
	// of a command's help asked for the other behaviour.
	wrapTail bool
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
// keeps a path copyable. A row asking for wrapTail is the one exception, and
// it breaks its tail between words rather than inside one, so the rule holds
// there too.
func formatRow(r row, window int) string {
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", r.indent))
	column := r.indent
	begins := r.indent
	for _, c := range r.cells {
		if displayWidth(c.text) < c.width {
			b.WriteString(pad(c.text, c.width))
			column += c.width
			begins += c.width
			continue
		}
		b.WriteString(c.text)
		b.WriteString("\n")
		column += c.width
		begins = continuation(column, r.indent, window)
		b.WriteString(strings.Repeat(" ", begins))
	}
	if r.wrapTail && window > 0 {
		b.WriteString(breakTail(r.tail, begins, window))
		return b.String()
	}
	b.WriteString(r.tail)
	return b.String()
}

// breakWords breaks text between words so that no line of it draws wider
// than room, indenting every line after the first to indent. It is the one
// word-breaker the renderer owns; breakTail below and the ceiling-bearing
// column both wrap through it rather than each carrying their own copy, so
// the rule that a word is never split lives in one place.
//
// A word wider than room is written whole and overruns, which is the rule
// the rest of the renderer follows and what keeps a reference inside a
// value copyable. Text with no room at all is written whole for the same
// reason, since breaking it a character at a time would help nobody.
func breakWords(text string, indent, room int) string {
	if room < 1 {
		return text
	}
	return packTokens(strings.Fields(text), indent, room)
}

// packTokens lays pre-split tokens out the way breakWords lays out the words
// of a string: as many tokens as fit on a line, joined by one space, moving
// to a new line indented to indent when the next token would not fit. A
// token wider than room on its own is written whole rather than split, which
// is the rule breakWords already follows for one overlong word.
//
// It exists so that the option-boundary wrap and the word wrap share one
// packing loop. The only difference between them is what counts as a token.
func packTokens(tokens []string, indent, room int) string {
	var b strings.Builder
	drawn := 0
	for _, tok := range tokens {
		width := displayWidth(tok)
		switch {
		case drawn == 0:
			b.WriteString(tok)
			drawn = width
		case drawn+1+width <= room:
			b.WriteString(" " + tok)
			drawn += 1 + width
		default:
			b.WriteString("\n")
			b.WriteString(strings.Repeat(" ", indent))
			b.WriteString(tok)
			drawn = width
		}
	}
	return b.String()
}

// breakTail breaks a tail between words so that no line of it reaches past
// the window, and indents every line after the first to the column the tail
// began at. It is breakWords with the room derived from where the tail
// began and the window it draws in, rather than a room stated directly.
func breakTail(text string, begins, window int) string {
	return breakWords(text, begins, window-begins)
}

// isOptionShaped reports whether a piece begins the way every option-group
// piece splitOnOptionBoundaries produces does: with `[`, `<`, or `-`. A
// piece beginning with an ordinary word character is free prose trailing
// after the option list ran out, not an option group, so a caller explodes
// it into its own words before packing and it keeps wrapping at the word
// level.
func isOptionShaped(piece string) bool {
	if piece == "" {
		return false
	}
	switch piece[0] {
	case '[', '<', '-':
		return true
	default:
		return false
	}
}

// breakOnOptions breaks text on option boundaries and packs the pieces, as
// many to a line as fit, indenting every line after the first to indent. An
// "option boundary" is a space next to an option group: before `[`, before
// `<`, before a bare `--`, or after a closing `]`. Each option group is kept
// whole rather than broken across a line end, so a reader's eye sees whole
// groups, and the packing keeps a row's continuation as short as the groups
// allow rather than spending one line per group.
//
// The first line is returned without a leading indent, so the caller can lay
// it on the row's first line in place; every line after it carries indent
// spaces at its head. A trailing piece that is free prose rather than an
// option group is exploded into its own words first, so a value whose option
// list runs out before its meaning does still wraps at the word level. A
// value with no option boundary at all is word-wrapped whole, the same shape
// breakTail already produces, so the rule degrades to the renderer's
// ordinary behaviour rather than silently changing it.
//
// A piece wider than room on its own is written whole, which is the rule
// breakWords already follows for one overlong word: packTokens's own
// start-of-line branch writes a token whole whatever its width, so a long
// command name needs no case of its own here.
func breakOnOptions(text string, indent, room int) string {
	if room < 1 {
		return text
	}
	pieces := splitOnOptionBoundaries(text)
	if len(pieces) <= 1 {
		return breakWords(text, indent, room)
	}
	last := len(pieces) - 1
	tokens := pieces
	if !isOptionShaped(pieces[last]) {
		tokens = append(append([]string{}, pieces[:last]...), strings.Fields(pieces[last])...)
	}
	return packTokens(tokens, indent, room)
}

// splitOnOptionBoundaries splits text on option boundaries and returns the
// pieces, with each piece kept whole. The boundary is the space character
// itself, which is then dropped: a boundary at position k means pieces[k]
// ends just before the space and pieces[k+1] starts with the option group
// (`[...`, `<...`, or `--...`). Text with no boundary returns the input
// as a single piece, so callers can tell "no boundaries found" apart from
// "one piece" by length.
//
// Boundaries that fall inside an open square-bracket group are skipped, so
// `[--state <state>]` is one chunk rather than two. A bracket group opens at
// any `[` and closes at the matching `]`, which is what keeps the rule's
// pieces readable: every option group stays whole, with the bracket's own
// internal `<` left alone rather than treated as the start of a fresh
// option. Opening on any `[` rather than only on `[--` is what lets a
// vocabulary group like `[new|get|set]` or a plain `[field]` split out as
// its own piece instead of gluing onto whatever came before it.
//
// A space also opens a boundary when the character before it closed a
// bracket group at depth zero, which is what separates a last option group
// from free prose following it. No command's syntax has that shape today,
// so this closes a gap in the rule rather than changing any command's
// rendering.
func splitOnOptionBoundaries(text string) []string {
	var pieces []string
	last := 0
	depth := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
			}
		case ' ':
			if depth > 0 {
				continue
			}
			j := i + 1
			closesBracket := i > 0 && text[i-1] == ']'
			switch {
			case closesBracket:
			case j < len(text) && text[j] == '[':
			case j < len(text) && text[j] == '<':
			case j+1 < len(text) && text[j] == '-' && text[j+1] == '-':
			default:
				continue
			}
			pieces = append(pieces, text[last:i])
			last = i + 1
		}
	}
	pieces = append(pieces, text[last:])
	return pieces
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

// renderSyntaxLine lays out a verb's syntax line for the help page: the line
// drawn whole when it fits the window, and broken on option boundaries (a
// space followed by `[--`, by `<`, or by a bare `--`) and indented under
// itself when it does not. The break is structural rather than visual, so a
// reader's eye sees one option group per line rather than a chunk of option
// glyphs scattered down the page.
//
// The width check lives here rather than at the call site because the rule
// that no caller outside the renderer and the table measures how wide text
// draws is what TestNoRowIsLaidOutOutsideTheOneRenderer enforces, and a
// syntax line that asks the renderer whether it fits is the same question
// raised at a different layer. A line shorter than the window is written
// whole, exactly as every help page draws one today; a longer line runs
// through breakOnOptions at indent, which falls back to breakWords when the
// syntax carries no option boundary, so the rule degrades to the word-wrap
// the rest of the renderer already uses. An unknown window (s.width == 0)
// renders the line whole, the same way every row does.
//
// The line is returned as one string with embedded newlines; the caller
// writes it through the same path as every other line of the block, so the
// page keeps its single-line-per-line shape.
func (s *session) renderSyntaxLine(text string, indent int) string {
	if s.width <= 0 || displayWidth(text) <= s.width {
		return text
	}
	return breakOnOptions(text, indent, s.width)
}

// row renders a row and writes it to stdout.
func (s *session) row(r row) {
	s.line(s.rowLine(r))
}
