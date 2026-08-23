package main

import (
	"runtime"
	"strings"
)

// tableColumn is one column of a table: the heading a reader sees above it,
// already rendered in the reader's language.
type tableColumn struct {
	// heading is what the column is called. It is a catalog entry rather than
	// a field name, since a reader has never met the field names.
	heading string
}

// tableRow is one line of a table: one field per column, in column order.
//
// A row may supply fewer fields than the table has columns. The last field it
// supplies then takes the rest of the line and widens no column, which is how
// a state offering nothing says so where the card reference would have been.
type tableRow struct {
	// section, when it carries text, starts a new group before this row: a
	// blank line, then the label, then the rows under it. The widths are still
	// chosen across the whole table, so every group's columns agree.
	section string
	// fields are the values, in column order.
	fields []string
	// note is free text printed under the row at no indent, which the comments
	// of dinah show need between one row and the next.
	note string
	// guides describes this row's place in a tree, one entry per level below
	// the top level the table draws, each saying whether the ancestor at that
	// level was the last of its siblings. The last entry describes this row's
	// own node. A row of no entries is a top-level row or an ordinary non-tree
	// row.
	guides []bool
}

// labelling says where a reader meets a table's column labels. It says nothing
// about whether the columns carry labels at all, since every value below keeps
// the headings on the columns and only moves where they are drawn.
type labelling int

const (
	// labelAbove draws the labels in a heading row over the columns, with a
	// rule under them, and draws them again in front of each field of the
	// stacked form. It is the zero value, so a table that says nothing about
	// its labels draws as it always has.
	labelAbove labelling = iota
	// labelInTheStack draws no heading row and no rule over the columns, and
	// labels each field of the stacked form the way labelAbove does. A reader
	// who can tell the columns apart at a glance needs no row of labels over
	// them, and the same reader meeting one field to a line does need a label
	// in front of each. The columns keep their headings under this value, so
	// the measure keeps its floors, the stacked form keeps its labels, and the
	// only line that goes is the row of labels over the columns.
	labelInTheStack
)

// table is a whole set of rows before layout. It owns what no row can decide
// for itself, which is how wide each column is, since that answer depends on
// every row in the set.
type table struct {
	indent  int
	columns []tableColumn
	rows    []tableRow
	// labels says where a reader meets this table's column labels. It defaults
	// to labelAbove, so a table that says nothing draws as it always has.
	labels labelling
	// wrapTail asks for the last column to be broken between words at the
	// window rather than written whole. It is an opt-in one table takes
	// rather than a change to how every table draws: a table measures against
	// assumedWindow whenever no width is stated, so wrapping by default would
	// rewrite the piped output of every listing the tool prints.
	wrapTail bool
	// ceilingColumn, together with hasCeiling, declares that column at half
	// the window the table draws in, breaking a value too wide for it
	// between words rather than measuring the column out to the value's own
	// width. It is an opt-in a table takes for itself, the same way
	// wrapTail is: every table that declares neither keeps measuring its
	// columns at the width its values need, exactly as every table drew
	// before this existed. The field after the capped column stays pinned
	// to the first line of the capped value, however many lines that value
	// wraps to, which is what keeps the listing scannable in a straight
	// column. Column 0 is a legitimate column to cap, so hasCeiling rather
	// than a negative sentinel is what tells "no ceiling" apart from
	// "column 0, capped". Only a table of exactly two columns, the capped
	// one first, draws through this; see ceilingRowLine.
	ceilingColumn int
	hasCeiling    bool
	// wrapOptions asks the capped column to break on option boundaries
	// (a space before `[`, before `<`, or before a bare `--`, and a space
	// after a closing `]`) before
	// falling back to word-wrap on the trailing prose. It is an opt-in a
	// table takes for itself, the same way wrapTail is: every table that
	// does not ask for it breaks the capped column on word boundaries, the
	// behaviour every ceiling-bearing table drew before this existed.
	wrapOptions bool
}

// tableGutter is how many display columns separate one column from the next.
// It is the integer rather than a string of spaces, so the padding is built
// once, in the row renderer, and nothing here carries a run of spaces of its
// own.
const tableGutter = 2

// assumedWindow is the width a table measures against when no documented
// source states one. The row renderer reads an unknown window as unbounded,
// which is right for a declared width and wrong for a measured one: unbounded
// makes a column as wide as the widest value ever seen, so the command list
// would start its summaries at display column 78 rather than at 41. A
// constant keeps a run whose window is unknown laying out the same way in
// every terminal and in a pipe.
const assumedWindow = 80

// ceilingContinuationIndent is how far a ceiling-bearing column's wrapped
// lines sit past its row's own indent: two columns past where the value
// itself starts, which in this table's one indent of two puts a wrapped
// line at four. It is a shallow, fixed indent rather than one that tracks
// the column's own width, in the same spirit as a manual page's option
// list: the line after the first says "this continues the value above",
// nothing more, and the field that follows the value stays on the first
// line regardless of how many lines the value itself needs.
const ceilingContinuationIndent = 2

// ruleGlyph is what a separator is drawn with. The table draws it rather than
// serving it from a catalog, so no translation can change it and no
// translation can misalign it.
const ruleGlyph = '-'

// The four glyphs a tree's guides are drawn from, in the same spirit as
// ruleGlyph: the table draws them and nothing outside this file does, so no
// translation can change them and no translation can misalign them. They are
// ASCII, and every piece composed from them is guideWidth columns wide, which
// is what keeps every column after the first lined up whatever the depth.
const (
	// guideTrunk continues an ancestor's line down the page.
	guideTrunk = '|'
	// guideRun reaches from an ancestor's line across to its child.
	guideRun = '-'
	// guideElbow turns the last of a set of siblings out of its parent's
	// line, which then stops.
	guideElbow = '`'
	// guideBlank is what a piece is filled with where no line is drawn.
	guideBlank = ' '
)

// guideWidth is how many display columns one level of a tree's guides takes.
const guideWidth = 4

// guidePiece builds one level of a row's prefix: the glyph the line starts
// with, then the fill, then the space that separates the piece from the next.
// It is built one column at a time the way rule is, so no run of padding is
// composed as a literal here.
func guidePiece(lead, fill rune) string {
	var b strings.Builder
	b.WriteRune(lead)
	for i := 0; i < guideWidth-2; i++ {
		b.WriteRune(fill)
	}
	b.WriteRune(guideBlank)
	return b.String()
}

// withGuides writes each row's tree prefix into its first field, so the prefix
// is measured as part of the value and every column after it lines up whatever
// the depth. A table whose rows carry no guides comes back unchanged.
//
// The rows it returns carry no guides of their own. The prefix has been folded
// into the field by then, so a second pass would draw it twice, and clearing
// the member is what says the folding has happened. Every later pass over a
// laid table reads the field.
func withGuides(t table) table {
	carried := false
	for _, r := range t.rows {
		if len(r.guides) > 0 {
			carried = true
			break
		}
	}
	if !carried {
		return t
	}
	rows := make([]tableRow, 0, len(t.rows))
	for _, r := range t.rows {
		fields := append([]string{}, r.fields...)
		if len(fields) > 0 {
			fields[0] = guidePrefix(r.guides) + fields[0]
		}
		rows = append(rows, tableRow{section: r.section, fields: fields, note: r.note})
	}
	return table{indent: t.indent, columns: t.columns, rows: rows, labels: t.labels}
}

// guidePrefix composes one row's prefix from its guides: a piece per ancestor
// level, then the join under the row's own parent.
//
// An ancestor that was the last of its siblings has no line left to draw, so
// its level is blank; every other ancestor's line carries on past this row.
func guidePrefix(guides []bool) string {
	var b strings.Builder
	for i, last := range guides {
		if i == len(guides)-1 {
			if last {
				b.WriteString(guidePiece(guideElbow, guideRun))
				continue
			}
			b.WriteString(guidePiece(guideTrunk, guideRun))
			continue
		}
		if last {
			b.WriteString(guidePiece(guideBlank, guideBlank))
			continue
		}
		b.WriteString(guidePiece(guideTrunk, guideBlank))
	}
	return b.String()
}

// listColumn is the one column of a block that prints a list under an indent
// rather than a table: one field per row, beneath a sentence that already says
// what the list is. It carries no heading, so the table prints neither a
// heading row nor a separator over it.
func listColumn() []tableColumn {
	return []tableColumn{{}}
}

// columns renders the heading of each named column of a block. The key is
// spelled column.<block>.<name>, which keeps the spelling in one place and
// keeps a call site from composing a key by hand.
func (s *session) columns(block string, names ...string) []tableColumn {
	rendered := make([]tableColumn, 0, len(names))
	for _, name := range names {
		rendered = append(rendered, tableColumn{heading: s.r.T("column." + block + "." + name)})
	}
	return rendered
}

// tableSiteRecorder is how the suite learns which call sites really drew a
// table. It is nil in a shipped binary and costs one comparison per table; the
// test that arms it pairs what the corpus reached against what an AST walk
// found, in both directions, so a site no test draws and a site no walk sees
// are both failures rather than silences.
var tableSiteRecorder func(file string, line int)

// recordTableSite reports its caller's caller, which is the call site of
// whichever of the two entry points below is running.
func recordTableSite() {
	if tableSiteRecorder == nil {
		return
	}
	if _, file, line, ok := runtime.Caller(2); ok {
		tableSiteRecorder(file, line)
	}
}

// table lays a table out and writes it to stdout.
func (s *session) table(t table) {
	recordTableSite()
	for _, line := range s.tableLines(t) {
		s.line(line)
	}
}

// tableLines lays a table out at the width this session draws at and returns
// its lines, the heading row first and the separator second.
//
// A table of no rows returns no lines at all, so the call site's own sentence
// about an empty listing is the whole answer. A table of one column returns
// its rows and neither a heading nor a separator, since one column under a
// sentence that already names it is a list. A table declaring labelInTheStack
// returns its rows without that pair too, keeping its columns' headings for
// the measure and for the stacked form.
//
// A table the window cannot hold returns a stack instead, one block per
// record, which stacks explains.
func (s *session) tableLines(t table) []string {
	recordTableSite()
	if len(t.rows) == 0 {
		return nil
	}
	laid := s.layOut(withGuides(t))
	if laid.stacks() {
		return laid.stackLines()
	}
	var lines []string
	for i, r := range laid.rows {
		if r.section != "" {
			lines = append(lines, "", r.section)
		}
		if i == 0 && len(laid.columns) > 1 && laid.labels == labelAbove {
			lines = append(lines, laid.headingLines()...)
		}
		lines = append(lines, splitLines(laid.rowLine(r))...)
		if r.note != "" {
			lines = append(lines, splitLines(strings.TrimSuffix(r.note, "\n"))...)
		}
	}
	return lines
}

// laidTable is a table with its empty columns removed and its widths chosen:
// everything layout needs, resolved once for the whole set of rows.
type laidTable struct {
	// indent is the display column every row starts at.
	indent int
	// labels is carried from the table, so line assembly need not ask twice.
	labels labelling
	// window is the width the table was measured against, which is
	// assumedWindow when no documented source stated one.
	window int
	// columns are the columns that survived the empty-column rule.
	columns []tableColumn
	// rows are the rows with the same columns removed and their trailing
	// empty fields dropped.
	rows []tableRow
	// widths are the chosen widths in display columns, one per column, with
	// no gutter in them. Every width but the last is what layout pads to; the
	// last is read by the separator alone, since the last field of a row is
	// never padded.
	widths []int
	// wrapTail is carried from the table, so the measure and the row assembly
	// both read one answer.
	wrapTail bool
	// ceilingColumn and hasCeiling are carried from the table, so the ceiling
	// pass and the row assembly both read one answer.
	ceilingColumn int
	hasCeiling    bool
	// wrapOptions is carried from the table, so the row assembly reads the
	// same answer the table declared.
	wrapOptions bool
}

// layOut removes the columns no row fills, chooses every column's width, and
// returns what the heading, separator and rows are all drawn from.
//
// The four passes run in one order and the order is load-bearing. The measure
// comes first, the near-miss widening second, the ceiling third, and the
// narrow-window backstop last, so that the backstop has the final word on how
// much of the window the columns before the last one may take. A
// ceiling-bearing column's width is a declaration rather than a further
// narrowing of the measure, so running it after the near-miss widening
// discards whatever that pass chose for it; the backstop can still narrow it
// further in a window too small even for half itself, which is why the
// ceiling has to run before the backstop rather than after.
func (s *session) layOut(t table) laidTable {
	window := s.width
	if window <= 0 {
		window = assumedWindow
	}
	laid := measure(t, window)
	if laid.hasCeiling {
		applyCeiling(&laid)
	}
	narrowToWindow(&laid)
	return laid
}

// halfWindow is the width a ceiling-bearing column draws at: half the
// window it draws in, rounded down. layOut has already replaced an unknown
// window with assumedWindow by the time this runs, so a piped run sees the
// same ceiling every time it draws.
func halfWindow(window int) int {
	return window / 2
}

// applyCeiling bounds a table's declared column at halfWindow, never below
// its own heading, since a heading is a floor every other pass in this file
// respects too. The ceiling is a bound rather than a width: a column whose
// own measured width already fits under it keeps that width, so a wide
// window puts the field after it beside the widest value in the column
// instead of stranding every row behind a river of blank space. Only a
// column measuring wider than the ceiling is capped, which is what lines a
// narrow window's rows up in one place. ceilingRowLine is what a value wider
// than the cap actually draws, wrapping between words rather than running
// past the column.
func applyCeiling(laid *laidTable) {
	c := laid.ceilingColumn
	if c < 0 || c >= len(laid.widths) {
		return
	}
	ceiling := halfWindow(laid.window)
	if natural := laid.widths[c]; natural < ceiling {
		ceiling = natural
	}
	if floor := displayWidth(laid.columns[c].heading); ceiling < floor {
		ceiling = floor
	}
	laid.widths[c] = ceiling
}

// measure runs the two passes before the backstop: it removes the columns no
// row fills, chooses every column's width, and takes back the near misses.
// What it returns is the table as the measure would have it, which is what the
// backstop then bounds and what the window is compared against.
//
// It is a step of layOut rather than a second way to lay a table out, and it
// is separate so that a test can hold the widths the measure chose beside the
// widths a reader gets. TestTheBackstopStandsAsideWhileTheRowsFit is that
// test.
func measure(t table, window int) laidTable {
	filled := withoutEmptyColumns(t)
	laid := laidTable{
		indent: filled.indent,
		// The labels are read off the table the caller declared rather than off
		// filled, which is the one field here that does not come from the
		// narrowed table. withoutEmptyColumns and withoutTrailingEmptyFields
		// each rebuild a table literal out of indent, columns and rows, so
		// filled.labels is the zero value on every path through them and would
		// silently put a labelInTheStack table's heading row back. wrapTail is
		// read off the declared table for the same reason: those two helpers
		// rebuild without it, so filled.wrapTail is false on every path and
		// would silently retire a wrapping table's opt-in.
		labels:        t.labels,
		window:        window,
		columns:       filled.columns,
		rows:          filled.rows,
		wrapTail:      t.wrapTail,
		ceilingColumn: t.ceilingColumn,
		hasCeiling:    t.hasCeiling,
		wrapOptions:   t.wrapOptions,
	}
	laid.widths = chooseWidths(laid)
	clearTheGutter(&laid)
	return laid
}

// clearTheGutter widens a column by the one column that decides whether the
// field after it is touched.
//
// A cell is padded to its column plus the gutter, and the row renderer gives a
// field the rest of its own line once it reaches that. A field exactly one
// column wider than what its column was measured at is the single case that
// falls between the two: it is too wide to leave the gutter after it and too
// narrow to take its own line, so it would print with one space behind it
// where every other field has two. Widening the column by one puts the field
// back inside it and gives the gutter back.
//
// Only a field the window ruled out of the measurement can land there, since
// this runs on the widths the measure chose and before the backstop narrows
// anything. Taking a near miss back costs one column and buys the row a line.
// The wide outlier the drop rule exists for is a different case and is left
// alone: the command list's 74-column syntax is nowhere near its column and
// still takes its own line.
//
// This runs before narrowToWindow and never after it. A column the backstop
// has narrowed holds values at every width between its floor and the width it
// was measured at, so a widening pass behind the backstop walks the column
// back out one value at a time and undoes the narrowing it was meant to keep.
func clearTheGutter(laid *laidTable) {
	for c := 0; c < len(laid.widths)-1; c++ {
		widened := true
		for widened {
			widened = false
			for _, r := range laid.rows {
				if c >= len(r.fields) || c == len(r.fields)-1 {
					continue
				}
				if displayWidth(r.fields[c]) == laid.widths[c]+1 {
					laid.widths[c]++
					widened = true
				}
			}
		}
	}
}

// withoutEmptyColumns drops every column no row carries a value in, heading
// and all, and returns the table that is left. A table asked for a column its
// rows never fill should print nothing rather than a heading over air, and a
// blank cell is indistinguishable from a rendering fault.
func withoutEmptyColumns(t table) table {
	kept := make([]int, 0, len(t.columns))
	for c := range t.columns {
		for _, r := range t.rows {
			if c < len(r.fields) && r.fields[c] != "" {
				kept = append(kept, c)
				break
			}
		}
	}
	if len(kept) == len(t.columns) {
		return withoutTrailingEmptyFields(t)
	}
	narrowed := table{indent: t.indent, rows: make([]tableRow, 0, len(t.rows))}
	for _, c := range kept {
		narrowed.columns = append(narrowed.columns, t.columns[c])
	}
	for _, r := range t.rows {
		fields := make([]string, 0, len(kept))
		for _, c := range kept {
			if c < len(r.fields) {
				fields = append(fields, r.fields[c])
			}
		}
		narrowed.rows = append(narrowed.rows, tableRow{
			section: r.section,
			fields:  fields,
			note:    r.note,
			guides:  r.guides,
		})
	}
	return withoutTrailingEmptyFields(narrowed)
}

// withoutTrailingEmptyFields ends every row at its last field carrying text.
// A row whose trailing fields are empty ends early rather than running past
// its own content, which is what removes the trailing run of spaces the state
// listing carried before this.
func withoutTrailingEmptyFields(t table) table {
	rows := make([]tableRow, 0, len(t.rows))
	for _, r := range t.rows {
		end := len(r.fields)
		for end > 0 && r.fields[end-1] == "" {
			end--
		}
		rows = append(rows, tableRow{
			section: r.section,
			fields:  r.fields[:end],
			note:    r.note,
			guides:  r.guides,
		})
	}
	return table{indent: t.indent, columns: t.columns, rows: rows}
}

// chooseWidths measures every column: the widest field under it together with
// its own heading, in display columns.
//
// Two things a row carries are left out of that measure. The last field a row
// supplies is never padded, so it widens nothing, and a field the window
// cannot hold does not get to widen its column either. The one exception is
// the last column, whose own values are read by the separator alone, and are
// counted here so that a rule sits under the whole of the column rather than
// under the heading word.
func chooseWidths(laid laidTable) []int {
	last := len(laid.columns) - 1
	widths := make([]int, len(laid.columns))
	for c, column := range laid.columns {
		widths[c] = displayWidth(column.heading)
	}
	for _, r := range laid.rows {
		dropped := fieldsOverWindow(r.fields, laid.indent, laid.window, laid.wrapTail)
		for c, field := range r.fields {
			if c == len(r.fields)-1 && c != last {
				continue
			}
			if dropped[c] {
				continue
			}
			if drawn := displayWidth(field); drawn > widths[c] {
				widths[c] = drawn
			}
		}
	}
	return widths
}

// fieldsOverWindow reports which of a row's fields do not get to widen their
// column, because the row cannot be laid out inside the window however the
// columns are chosen.
//
// Add the row's fields up packed tight, each at its own width with one gutter
// between neighbours, plus the indent. While that total is over the window and
// more than one field is left, drop the widest field and add the rest up
// again, taking the leftmost on a tie. A dropped field is still printed: it
// reaches its column at layout, takes the rest of its own line, and the fields
// after it resume on a continuation line.
//
// A row's last field is never a candidate. It widens no column, so dropping it
// removes nothing from the measurement, and all it can do is stop the loop
// early and leave a genuinely over-wide field in place.
//
// A table that wraps its tail is measured with the last field counted at
// minTailColumns rather than at its own width, because a tail that breaks at
// the window imposes no width on the row: counting it whole would drop the
// field before it out of the measurement and collapse a column a reader can
// see is wider than its heading.
func fieldsOverWindow(fields []string, indent, window int, wrapsTail bool) []bool {
	dropped := make([]bool, len(fields))
	last := len(fields) - 1
	for {
		total := indent
		counted := 0
		for i, field := range fields {
			if dropped[i] {
				continue
			}
			if counted > 0 {
				total += tableGutter
			}
			total += countedWidth(field, i == last, wrapsTail)
			counted++
		}
		if total <= window || counted <= 1 {
			return dropped
		}
		widest, at := 0, -1
		for i := 0; i < len(fields)-1; i++ {
			if dropped[i] {
				continue
			}
			if drawn := displayWidth(fields[i]); drawn > widest {
				widest, at = drawn, i
			}
		}
		if at < 0 {
			return dropped
		}
		dropped[at] = true
	}
}

// countedWidth is how wide a field counts toward the measurement that decides
// which fields are too wide for the window. A field counts at its own width,
// except the last field of a table that wraps its tail, which counts at the
// room a continuation always keeps for itself.
func countedWidth(field string, isLast, wrapsTail bool) int {
	drawn := displayWidth(field)
	if isLast && wrapsTail && drawn > minTailColumns {
		return minTailColumns
	}
	return drawn
}

// narrowToWindow is the backstop a narrow window needs. While the indent plus
// every column but the last, each with its gutter, leaves less of the window
// than tailRoom asks for, narrow the widest column that is still above its own
// heading's width, leftmost on a tie. It stops when no column can be narrowed,
// since a heading is a floor.
//
// The last column keeps whatever width it measured, because narrowing a column
// nothing is ever padded to would change nothing a reader sees except the
// length of one rule.
//
// This is the last pass of layOut, so what it leaves is what a reader gets:
// either the columns before the last one leave tailRoom of the window after
// them, or every one of them stands at its own heading and the window is
// narrower than the headings alone.
// TestTheBackstopHoldsWhateverTheWidthsWere asserts exactly that pair over the
// laid-out table, and TestTheBackstopStandsAsideWhileTheRowsFit asserts the
// other half, that it narrows nothing while the window can hold the widths the
// measure chose.
func narrowToWindow(laid *laidTable) {
	room := tailRoom(*laid)
	for {
		lead := laid.indent
		for c := 0; c < len(laid.widths)-1; c++ {
			lead += laid.widths[c] + tableGutter
		}
		if lead+room <= laid.window {
			return
		}
		widest, at := 0, -1
		for c := 0; c < len(laid.widths)-1; c++ {
			floor := displayWidth(laid.columns[c].heading)
			if laid.widths[c] <= floor {
				continue
			}
			if laid.widths[c] > widest {
				widest, at = laid.widths[c], c
			}
		}
		if at < 0 {
			return
		}
		laid.widths[at]--
	}
}

// tailRoom is how much of the window the columns before the last one have to
// leave after them: what the last column measured, or minTailColumns,
// whichever is smaller.
//
// The cap is what a narrow window needs. A last column holding a path or a
// summary measures far more than any narrow window can give it, so reserving
// its measured width would narrow every column to its heading and buy the
// reader nothing; minTailColumns is the width below which no layout helps, so
// that is where the reservation stops.
//
// Reading the measured width is what this pass got wrong before.
// minTailColumns stood in for the tail's need back when the last column had no
// width of its own. It has one now, so that the separator can draw a rule
// under the whole column, and a flat reservation fires the backstop by the
// difference between the two: a listing whose last column measures five
// columns was squeezed fifteen columns of window before it was under any
// pressure, and broken apart eleven columns before it had to be.
func tailRoom(laid laidTable) int {
	if len(laid.widths) == 0 {
		return minTailColumns
	}
	measured := laid.widths[len(laid.widths)-1]
	if measured < minTailColumns {
		return measured
	}
	// A tail the renderer can break between words gives up nothing to the flat
	// reservation: whatever does not fit wraps onto the next line and the
	// reader still reads it. A tail whose every value is one unbreakable token
	// has no such give, so a reservation short of what it measured is a line
	// that runs past the window rather than a line that wraps, which is the
	// one case the flat reservation gets wrong.
	//
	// Such a tail asks for its whole width, and gets it wherever the columns
	// ahead of it can still stand at their own headings, which is the floor
	// the backstop refuses to narrow past anyway, so nothing is given up by
	// asking. A token too wide for even that keeps the flat reservation, since
	// reserving its measure would narrow every column ahead of it to its
	// heading and still not fit.
	if !tailIsUnbreakable(laid) {
		return minTailColumns
	}
	lead := laid.indent
	for c := 0; c < len(laid.widths)-1; c++ {
		lead += displayWidth(laid.columns[c].heading) + tableGutter
	}
	if lead+measured <= laid.window {
		return measured
	}
	return minTailColumns
}

// tailIsUnbreakable reports whether every value of the last column is a single
// word, so that no line break inside one is available to the row renderer. A
// column of refusal names is the shape this answers true for; a column of
// prose or of paths with spaces in them is the shape it answers false for.
func tailIsUnbreakable(laid laidTable) bool {
	last := len(laid.widths) - 1
	if last < 0 {
		return false
	}
	for _, r := range laid.rows {
		if last >= len(r.fields) {
			continue
		}
		if strings.ContainsAny(r.fields[last], " 	") {
			return false
		}
	}
	return true
}

// stacks reports whether a column stands at its own heading and still cannot
// hold a field under it, which is the point at which the table becomes a
// stack.
//
// Both halves of that carry weight. A field reaching its column takes the rest
// of its own line and the fields after it resume underneath, which is the
// staircase a reader sees go wrong. A column standing at its heading is what
// separates a window too narrow for the table from a single field too wide for
// any window: a column above its heading was given room and chose to let one
// outlier overflow, which is what the drop rule in chooseWidths exists to
// produce and what the command list of bare dinah relies on at eighty columns,
// while a column already at its heading has nothing left to give.
//
// A block of one column never stacks. It carries no heading, so it has no
// label to draw and no column that can stand at one.
//
// A capped column is the one column a field cannot reach past, whatever the
// window leaves it, because ceilingRowLine wraps its value down its own
// track instead of letting one value take the rest of the line. The
// staircase this rule exists to catch cannot form there, so the capped
// column never puts a table into a stack. Without that exemption a window
// narrow enough to leave the cap standing at its own heading stacks a table
// that would have wrapped perfectly well, which is what the command listing
// did between 32 and 33 columns: the stacked form draws every value whole,
// so the escape from a 33-column window was a run of 87-column lines.
func (laid laidTable) stacks() bool {
	if len(laid.columns) < 2 {
		return false
	}
	for _, r := range laid.rows {
		for c, field := range r.fields {
			if c == len(r.fields)-1 {
				continue
			}
			if laid.hasCeiling && c == laid.ceilingColumn {
				continue
			}
			if laid.widths[c] > displayWidth(laid.columns[c].heading) {
				continue
			}
			if displayWidth(field) >= laid.widths[c]+tableGutter {
				return true
			}
		}
	}
	return false
}

// stackLines draws each record as its own block, one field to a line, with the
// column's heading in front of the value as its label.
//
// A field holding no text draws no line, since a label over nothing tells a
// reader nothing. Every label is padded to the widest heading in the table, so
// every value in the stack begins at one display column, across records as
// well as within one. One blank line separates one record from the next, and a
// section replaces the blank line the record it opens would otherwise have
// drawn.
//
// Neither the heading row nor the separator row is drawn. Each label carries
// its own heading, and a stack has no columns for a rule to trace.
func (laid laidTable) stackLines() []string {
	label := 0
	for _, column := range laid.columns {
		if drawn := displayWidth(column.heading); drawn > label {
			label = drawn
		}
	}
	var lines []string
	for i, r := range laid.rows {
		switch {
		case r.section != "":
			lines = append(lines, "", r.section)
		case i > 0:
			lines = append(lines, "")
		}
		for c, field := range r.fields {
			if field == "" {
				continue
			}
			lines = append(lines, splitLines(laid.stackLine(laid.columns[c].heading, label, field))...)
		}
		if r.note != "" {
			lines = append(lines, splitLines(strings.TrimSuffix(r.note, "\n"))...)
		}
	}
	return lines
}

// stackLine lays one labelled line out through the row renderer, as a row of
// one cell holding the label and a tail holding the value.
//
// The line never breaks. The label is padded to the widest heading in the
// table, so it is always narrower than the cell it sits in, and the value
// takes the tail, which the row renderer never breaks. A value wider than what
// is left of the window wraps in the terminal, exactly as the last field of a
// table row does.
func (laid laidTable) stackLine(heading string, label int, value string) string {
	built := row{
		indent: laid.indent,
		cells:  []cell{{text: heading, width: label + tableGutter}},
		tail:   value,
	}
	return formatRow(built, laid.window)
}

// headingLines returns the heading row and the separator row under it.
func (laid laidTable) headingLines() []string {
	headings := make([]string, 0, len(laid.columns))
	rules := make([]string, 0, len(laid.columns))
	start := laid.indent
	for c, column := range laid.columns {
		headings = append(headings, column.heading)
		rules = append(rules, rule(laid.ruleWidth(c, start)))
		start += laid.widths[c] + tableGutter
	}
	return []string{laid.rowLine(tableRow{fields: headings}), laid.rowLine(tableRow{fields: rules})}
}

// ruleWidth is how wide the rule under one column draws: its column's own
// width, or the columns left between where the column starts and the right
// edge of the window, whichever is smaller, and never fewer than one.
//
// The clamp shortens a rule and changes no other line. A value the window
// cannot hold is laid out exactly as it is without the clamp, so a single
// unbroken value with nowhere to wrap runs past the edge with its rule
// stopping short of it.
func (laid laidTable) ruleWidth(column, start int) int {
	width := laid.widths[column]
	if room := laid.window - start; room < width {
		width = room
	}
	if width < 1 {
		return 1
	}
	return width
}

// rowLine lays one row out: every field but the last padded to its column and
// the gutter after it, and the last field taking whatever is left of the
// line. A ceiling-bearing table draws through ceilingRowLine instead, which
// keeps the field after the capped column on the first line rather than
// letting it resume under whichever column the capped value happened to
// reach.
//
// A wrapping table stamps wrapTail onto its heading row and its rule row as
// well as onto its body, and both survive it. breakTail breaks a tail between
// words, so a rule row, whose tail is one unbroken run of glyphs, comes back
// the length it went in however narrow the window. A column label wide enough
// to reach the edge would wrap where a body cell wraps, which is the layout
// this table asked for rather than a fault in it.
func (laid laidTable) rowLine(r tableRow) string {
	if laid.hasCeiling {
		return laid.ceilingRowLine(r)
	}
	built := row{indent: laid.indent, wrapTail: laid.wrapTail}
	for c, field := range r.fields {
		if c == len(r.fields)-1 {
			built.tail = field
			break
		}
		built.cells = append(built.cells, cell{text: field, width: laid.widths[c] + tableGutter})
	}
	return formatRow(built, laid.window)
}

// ceilingRowLine lays out a row of a ceiling-bearing table: the capped
// column breaks between words rather than running past its own width, and
// the field after it stays on the row's first line however many lines the
// capped value needs, which is what keeps a reader's eye running down the
// summaries in a straight line rather than chasing them down the page.
//
// The field after the capped column wraps too, when the table declares
// wrapTail. The two wraps run down the page together: physical line N
// carries the capped column's own Nth line beside the field's own Nth line,
// so each column reads straight down its own track and neither is pushed
// below the other. A line whose field has already run out is written with no
// trailing pad, so a syntax continuation standing alone carries no invisible
// spaces after it.
//
// This assumes the shape hasCeiling exists for: exactly two columns, the
// capped one first and an unpadded field after it. A table asking for a
// ceiling on any other shape is not a case this draws correctly, and none
// of this tool's tables ask for one.
//
// A value whose very first word is wider than the column on its own is the
// one case wrapping cannot help: breakWords writes that word whole rather
// than splitting it, exactly as breakTail already does for a tail, so the
// same word can still reach past the cap. Such a row falls back to the
// shape a capped overflow drew before wrapping existed: the whole capped
// value on its own lines, and the field after it on one further line of
// its own (wrapped the same way, if the table asks for it), rather than
// fighting the first line for room the word cannot leave it.
func (laid laidTable) ceilingRowLine(r tableRow) string {
	c := laid.ceilingColumn
	room := laid.widths[c]
	wrapIndent := laid.indent + ceilingContinuationIndent
	value := r.fields[c]
	after := ""
	if len(r.fields) > c+1 {
		after = r.fields[c+1]
	}
	wrap := func(indent int) string {
		w := breakWords(value, indent, room)
		if laid.wrapOptions && displayWidth(value) > room {
			w = breakOnOptions(value, indent, room)
		}
		return w
	}

	// A single word wider than the cap on its own is the one case wrapping
	// cannot help: breakWords writes it whole, exactly as breakTail already
	// does for a tail, so the first line can still overrun room. There, the
	// value draws on its own line or lines and the field after it follows on
	// one further line of its own, wrapped the same way a wrapping table's
	// own tail wraps, rather than fighting the value for room its own first
	// word already used up.
	first, _, _ := strings.Cut(wrap(wrapIndent), "\n")
	if displayWidth(first) > room {
		whole := formatRow(row{indent: laid.indent, tail: wrap(wrapIndent)}, laid.window)
		trailer := formatRow(row{indent: wrapIndent, tail: after, wrapTail: laid.wrapTail}, laid.window)
		return whole + "\n" + trailer
	}

	// Both axes are wrapped into their own list of lines, with no indent of
	// their own, and the loop below puts each line where it belongs. The
	// field's first line keeps the whole room its own column leaves it; its
	// continuations draw ceilingContinuationIndent columns further right
	// and are wrapped that much narrower, so the offset is a hanging indent
	// rather than the same room slid sideways past the window's edge.
	syntaxLines := splitLines(wrap(0))
	summaryLines := []string{after}
	if laid.wrapTail && laid.window > 0 {
		begins := laid.indent + room + tableGutter
		summaryLines = splitLines(breakWordsHanging(after, 0, laid.window-begins, ceilingContinuationIndent))
	}

	lines := len(syntaxLines)
	if len(summaryLines) > lines {
		lines = len(summaryLines)
	}
	var b strings.Builder
	for i := 0; i < lines; i++ {
		if i > 0 {
			b.WriteString("\n")
		}
		var syntax, summary string
		if i < len(syntaxLines) {
			syntax = syntaxLines[i]
		}
		if i < len(summaryLines) {
			summary = summaryLines[i]
		}
		indent := laid.indent
		if i > 0 {
			indent = wrapIndent
		}
		if summary == "" {
			// Nothing follows on this line, so the syntax continuation goes
			// through as a tail rather than as a padded cell, which is what
			// keeps a line whose field has already run out free of trailing
			// spaces. It is still the row renderer that draws it.
			b.WriteString(formatRow(row{indent: indent, tail: syntax}, laid.window))
			continue
		}
		b.WriteString(formatRow(row{indent: indent, cells: []cell{{text: syntax, width: room + tableGutter}}, tail: summary}, laid.window))
	}
	return b.String()
}

// rule returns a separator's run of glyphs, built one column at a time so that
// no run of padding is composed outside the row renderer.
func rule(width int) string {
	var b strings.Builder
	for i := 0; i < width; i++ {
		b.WriteRune(ruleGlyph)
	}
	return b.String()
}

// splitLines breaks a laid-out row into the lines it draws, since a field that
// reaches its column takes the rest of its own line and resumes underneath.
func splitLines(text string) []string {
	return strings.Split(text, "\n")
}
