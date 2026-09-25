package main

import (
	"runtime"
	"strings"

	"dinah/internal/screen"
	"dinah/internal/textwidth"
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
// a column offering nothing says so where the card reference would have been.
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
	// ruleAbove draws the separator a heading row draws under itself above
	// this row as well, which is how a total is set off from the rows it
	// sums. The stacked form draws no such rule, since each record there
	// already stands on its own.
	ruleAbove bool
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
	// stackOnOverflow makes a table stack when narrowToWindow shortens a
	// non-final column below a row value. Listings opt in so a continuation
	// never begins without its reference.
	stackOnOverflow bool
	// cutTail asks for the last column to be cut to the room the window
	// leaves it, ending in an ellipsis, rather than written whole. It is an
	// opt-in one table takes for itself, on the terms wrapTail and
	// hasCeiling are, and it cuts only where the window's width is known:
	// a piped run with no stated width keeps every value whole, so a script
	// reading the human form never loses text. Where the room left is under
	// minTailColumns nothing is cut.
	cutTail bool
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
		rows = append(rows, tableRow{section: r.section, fields: fields, note: r.note, ruleAbove: r.ruleAbove})
	}
	return table{
		indent: t.indent, columns: t.columns, rows: rows, labels: t.labels,
		wrapTail: t.wrapTail, ceilingColumn: t.ceilingColumn, hasCeiling: t.hasCeiling,
		wrapOptions: t.wrapOptions, stackOnOverflow: t.stackOnOverflow, cutTail: t.cutTail,
	}
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

// indentedLine lays a single row out at an indent of two display columns,
// with no cell of its own: the whole text is the row's tail. It is what a
// caller outside this file reaches for instead of building a row.row or
// calling rowLine/formatRow directly, which only this file may do (pattern
// 9, guards_test.go).
func (s *session) indentedLine(text string) string {
	return s.rowLine(row{indent: 2, tail: text})
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
		if r.ruleAbove {
			lines = append(lines, laid.ruleLine())
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
	// stackOnOverflow is the table's opt-in carried through layout.
	stackOnOverflow bool
	// cutTail is carried from the table, so the measure and the cut both
	// read one answer.
	cutTail bool
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
	if laid.cutTail && s.width > 0 {
		cutTheTail(&laid)
	}
	return laid
}

// tailEllipsis is what a cut value ends in: one display column standing for
// the text the cut removed.
const tailEllipsis = "\u2026"

// cutTheTail cuts every last field wider than the room the window leaves the
// last column, so that no row reaches past the window. A cut value keeps as
// much of its text as fits in the room less one column and ends in
// tailEllipsis. The cut falls between runes and is measured in display
// columns, so a title of wide characters is cut where it draws rather than
// where it counts. Where the room is under minTailColumns nothing is cut, and
// the table draws by the rules it already follows.
func cutTheTail(laid *laidTable) {
	last := len(laid.widths) - 1
	lead := laid.indent
	for c := 0; c < last; c++ {
		lead += laid.widths[c] + tableGutter
	}
	room := laid.window - lead
	if room < minTailColumns {
		return
	}
	rows := make([]tableRow, 0, len(laid.rows))
	for _, r := range laid.rows {
		if len(r.fields) == last+1 && displayWidth(r.fields[last]) > room {
			fields := append([]string{}, r.fields...)
			fields[last] = cutToColumns(fields[last], room-displayWidth(tailEllipsis)) + tailEllipsis
			r.fields = fields
		}
		rows = append(rows, r)
	}
	laid.rows = rows
	if laid.widths[last] > room {
		laid.widths[last] = room
	}
}

// cutToColumns is the longest leading run of whole runes drawing no wider
// than the given number of display columns.
func cutToColumns(text string, columns int) string {
	var kept strings.Builder
	drawn := 0
	for _, r := range text {
		width := displayWidth(string(r))
		if drawn+width > columns {
			break
		}
		drawn += width
		kept.WriteRune(r)
	}
	return kept.String()
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
		labels:          t.labels,
		window:          window,
		columns:         filled.columns,
		rows:            filled.rows,
		wrapTail:        t.wrapTail,
		ceilingColumn:   t.ceilingColumn,
		hasCeiling:      t.hasCeiling,
		wrapOptions:     t.wrapOptions,
		stackOnOverflow: t.stackOnOverflow,
		cutTail:         t.cutTail,
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
			section:   r.section,
			fields:    fields,
			note:      r.note,
			guides:    r.guides,
			ruleAbove: r.ruleAbove,
		})
	}
	return withoutTrailingEmptyFields(narrowed)
}

// withoutTrailingEmptyFields ends every row at its last field carrying text.
// A row whose trailing fields are empty ends early rather than running past
// its own content, which is what removes the trailing run of spaces the column
// listing carried before this.
func withoutTrailingEmptyFields(t table) table {
	rows := make([]tableRow, 0, len(t.rows))
	for _, r := range t.rows {
		end := len(r.fields)
		for end > 0 && r.fields[end-1] == "" {
			end--
		}
		rows = append(rows, tableRow{
			section:   r.section,
			fields:    r.fields[:end],
			note:      r.note,
			guides:    r.guides,
			ruleAbove: r.ruleAbove,
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
		// A tail the table cuts imposes no width on the row, for the reason
		// a wrapped one imposes none, so it is counted the same way.
		dropped := fieldsOverWindow(r.fields, laid.indent, laid.window, laid.wrapTail || laid.cutTail)
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
	// A tail the table cuts gives up whatever does not fit, so it asks for no
	// more than the flat reservation whatever its values are made of.
	if laid.cutTail {
		return minTailColumns
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
			// A table that stacks on overflow stacks the moment a field is
			// wider than its column. A field one or two columns wider leaves
			// less than a full gutter before the next field rather than
			// reaching it, and a row drawn that way reads as two fields run
			// together.
			if laid.stackOnOverflow && displayWidth(field) > laid.widths[c] {
				return true
			}
			if !laid.stackOnOverflow && laid.widths[c] > displayWidth(laid.columns[c].heading) {
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
	for _, column := range laid.columns {
		headings = append(headings, column.heading)
	}
	return []string{laid.rowLine(tableRow{fields: headings}), laid.ruleLine()}
}

// ruleLine returns the separator row: a rule under every column, as wide as
// the column, with the gutter left blank between neighbouring rules.
func (laid laidTable) ruleLine() string {
	rules := make([]string, 0, len(laid.columns))
	start := laid.indent
	for c := range laid.columns {
		rules = append(rules, rule(laid.ruleWidth(c, start)))
		start += laid.widths[c] + tableGutter
	}
	return laid.rowLine(tableRow{fields: rules})
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

// boardRule is a rule under a column of the board: width copies of the glyph
// the board's glyph set draws rules with, built one column at a time the way
// rule is, because the glyph is an argument here rather than ruleGlyph.
func boardRule(glyph rune, width int) string {
	var b strings.Builder
	for i := 0; i < width; i++ {
		b.WriteRune(glyph)
	}
	return b.String()
}

// boardMinimumColumn is the narrowest column the board draws, M in the
// specification's fit rules, except in a window too narrow for one column of
// it.
const boardMinimumColumn = 20

// boardTitleIndent is how far a card's title line sits under its first line.
const boardTitleIndent = 2

// boardSlotGap is how many columns separate a card's number from its holder
// and its holder from its priority.
const boardSlotGap = 2

// boardSlotMinimum is the narrowest room a cut holder is drawn in. Below it
// the holder is dropped rather than cut to a stub nobody could read.
const boardSlotMinimum = 4

// boardGlyphs is one glyph set: what each state, the operator mark, a rule
// and a cut draw with. A glyph is text rather than a rune because the plain
// set's cut is three characters.
type boardGlyphs struct {
	ready, active, blocked, operator string
	rule                             rune
	ellipsis                         string
}

// boardColumn is one column of one section of a board, as board.go chose it:
// what its heading says and the cards it shows, already capped.
type boardColumn struct {
	// title is the column's title.
	title string
	// operator marks a column owned by the operator.
	operator bool
	// count is the column's count, already rendered.
	count string
	// cards are the cards the column shows, in the section's order.
	cards []boardCard
	// more is the line counting the cards not shown, empty when every card
	// is shown.
	more string
}

// boardCard is one card of a board column, as board.go chose it.
type boardCard struct {
	// glyph is the state's glyph and colour its colour.
	glyph  string
	colour screen.Colour
	// number is the card's reference without the workbench's slug.
	number string
	// mark is true when an item on the card waits on the operator.
	mark bool
	// holder is the holder of an active card or the kind of a blocked one's
	// block, and priority is the card's priority; either may be empty.
	holder, priority string
	// title is the card's title.
	title string
}

// colourSpan is a run of a drawn line's bytes to be drawn in a colour.
type colourSpan struct {
	start, end int
	colour     screen.Colour
}

// drawnLine is one line of a board: its text, laid out in plain text, and
// the spans of it to colour. Colour is attached after layout, so nothing the
// measure reads ever carries a control sequence.
type drawnLine struct {
	text  string
	spans []colourSpan
}

// segments splits a drawn line into the segments the terminal layer writes,
// so that the text of the segments joined is the line's text byte for byte.
func (d drawnLine) segments() []screen.Segment {
	var segments []screen.Segment
	at := 0
	for _, span := range d.spans {
		if span.start > at {
			segments = append(segments, screen.Segment{Text: d.text[at:span.start]})
		}
		segments = append(segments, screen.Segment{Text: d.text[span.start:span.end], Colour: span.colour})
		at = span.end
	}
	if at < len(d.text) {
		segments = append(segments, screen.Segment{Text: d.text[at:]})
	}
	return segments
}

// shifted is a line's spans moved right by offset bytes, which is where they
// fall once the line is placed after offset bytes of other text.
func shifted(spans []colourSpan, offset int) []colourSpan {
	moved := make([]colourSpan, 0, len(spans))
	for _, span := range spans {
		moved = append(moved, colourSpan{start: offset + span.start, end: offset + span.end, colour: span.colour})
	}
	return moved
}

// boardBands answers how many columns each band of a section holds and how
// wide every one of them is, for visible columns drawn in draw display
// columns. As many columns fit in a band as fit at the minimum width with a
// gutter between each, never fewer than one, and every column of the section
// shares one width so the columns of later bands sit under the first's.
func boardBands(visible, draw int) (perBand, width int) {
	fit := (draw + tableGutter) / (boardMinimumColumn + tableGutter)
	if fit < 1 {
		fit = 1
	}
	perBand = visible
	if perBand > fit {
		perBand = fit
	}
	if perBand < 1 {
		perBand = 1
	}
	width = (draw+tableGutter)/perBand - tableGutter
	return perBand, width
}

// cutText cuts a value to room display columns: the longest leading run of
// whole units that fits in the room less the ellipsis, then the ellipsis.
// Where the room is narrower than the ellipsis itself, the cut keeps whole
// units with no ellipsis, since an ellipsis wider than its room would put
// the line past it, and it drops any space the run ends in, since nothing
// follows it.
func cutText(text string, room int, ellipsis string) string {
	if displayWidth(text) <= room {
		return text
	}
	return cutPrefix(text, room, ellipsis) + cutMark(room, ellipsis)
}

// cutPrefix is the part of a cut value kept from the value itself.
func cutPrefix(text string, room int, ellipsis string) string {
	mark := displayWidth(ellipsis)
	if room < mark {
		return strings.TrimRight(textwidth.Cut(text, room), " ")
	}
	return textwidth.Cut(text, room-mark)
}

// cutMark is what a cut value ends in: the ellipsis, where the room holds it.
func cutMark(room int, ellipsis string) string {
	if room < displayWidth(ellipsis) {
		return ""
	}
	return ellipsis
}

// cutLine cuts a drawn line to room, keeping the colour of every glyph that
// survives the cut. Every span the board colours is one glyph, which is one
// whole unit, and the cut falls between whole units, so a span is either
// kept whole or dropped whole.
func cutLine(line drawnLine, room int, ellipsis string) drawnLine {
	if displayWidth(line.text) <= room {
		return line
	}
	kept := cutPrefix(line.text, room, ellipsis)
	cut := drawnLine{text: kept + cutMark(room, ellipsis)}
	for _, span := range line.spans {
		if span.end <= len(kept) {
			cut.spans = append(cut.spans, span)
		}
	}
	return cut
}

// boardLines lays out one section of a board drawn in draw display columns:
// its visible columns dealt into bands, a blank line between bands. No line
// is wider than draw and none ends in a space.
func boardLines(columns []boardColumn, draw int, glyphs boardGlyphs) []drawnLine {
	perBand, width := boardBands(len(columns), draw)
	var lines []drawnLine
	for start := 0; start < len(columns); start += perBand {
		end := start + perBand
		if end > len(columns) {
			end = len(columns)
		}
		if start > 0 {
			lines = append(lines, drawnLine{})
		}
		var laid [][]drawnLine
		for _, column := range columns[start:end] {
			laid = append(laid, boardColumnLines(column, width, glyphs))
		}
		lines = append(lines, boardBand(laid, width)...)
	}
	return lines
}

// boardBand lays the columns of one band side by side. Line i of the band is
// line i of each column, a column shorter than the band contributing
// nothing. Every column up to the last one contributing a non-empty line is
// a cell of the column's width and a gutter, and that last one is the tail,
// so no line trails padding. Every cell's text is narrower than its cell, so
// formatRow pads each in place and never takes its overflow branch, and a
// span's offset in the line is the length of the row cut before its cell.
func boardBand(columns [][]drawnLine, width int) []drawnLine {
	height := 0
	for _, column := range columns {
		if len(column) > height {
			height = len(column)
		}
	}
	lineOf := func(column []drawnLine, i int) drawnLine {
		if i < len(column) {
			return column[i]
		}
		return drawnLine{}
	}
	var lines []drawnLine
	for i := 0; i < height; i++ {
		last := -1
		for k, column := range columns {
			if lineOf(column, i).text != "" {
				last = k
			}
		}
		if last < 0 {
			lines = append(lines, drawnLine{})
			continue
		}
		cells := make([]cell, 0, last)
		for k := 0; k < last; k++ {
			cells = append(cells, cell{text: lineOf(columns[k], i).text, width: width + tableGutter})
		}
		tail := lineOf(columns[last], i).text
		laid := drawnLine{text: formatRow(row{cells: cells, tail: tail}, 0)}
		for k := 0; k <= last; k++ {
			offset := len(formatRow(row{cells: cells[:k]}, 0))
			laid.spans = append(laid.spans, shifted(lineOf(columns[k], i).spans, offset)...)
		}
		lines = append(lines, laid)
	}
	return lines
}

// boardColumnLines lays out one column at width: its heading, its rule, two
// lines per card and the line counting the cards not shown. Every line is
// cut to width and none ends in a space.
func boardColumnLines(column boardColumn, width int, glyphs boardGlyphs) []drawnLine {
	lines := []drawnLine{boardHeadingCell(column, width, glyphs), {text: boardRule(glyphs.rule, width)}}
	for _, card := range column.cards {
		lines = append(lines, boardCardLine(card, width, glyphs), boardTitleLine(card.title, width, glyphs))
	}
	if column.more != "" {
		lines = append(lines, drawnLine{text: cutText(column.more, width, glyphs.ellipsis)})
	}
	return lines
}

// boardHeadingCell is a column's heading: its title, the operator mark where
// the column is owned by the operator, and its count. The mark and the count
// are kept whole and the title alone is cut, unless even they do not fit, in
// which case the whole heading is cut.
func boardHeadingCell(column boardColumn, width int, glyphs boardGlyphs) drawnLine {
	suffix := drawnLine{text: " "}
	if column.operator {
		suffix.spans = []colourSpan{{start: 1, end: 1 + len(glyphs.operator), colour: screen.Yellow}}
		suffix.text += glyphs.operator + " "
	}
	suffix.text += column.count
	whole := drawnLine{text: column.title + suffix.text, spans: shifted(suffix.spans, len(column.title))}
	if displayWidth(whole.text) <= width {
		return whole
	}
	room := width - displayWidth(suffix.text)
	title := cutText(column.title, room, glyphs.ellipsis)
	if room < 1 || title == "" {
		return cutLine(whole, width, glyphs.ellipsis)
	}
	return drawnLine{text: title + suffix.text, spans: shifted(suffix.spans, len(title))}
}

// boardCardLine is a card's first line: its glyph, its number and its mark,
// then its holder and its priority, each after a gap, for as many of them as
// fit. It tries each form in turn and takes the first that fits: both slots,
// the holder alone, the holder cut to what is left while at least
// boardSlotMinimum columns are, and neither. Where even the glyph, the number
// and the mark do not fit, they are cut.
func boardCardLine(card boardCard, width int, glyphs boardGlyphs) drawnLine {
	lead := drawnLine{text: card.glyph + " " + card.number}
	if card.colour != screen.None {
		lead.spans = append(lead.spans, colourSpan{start: 0, end: len(card.glyph), colour: card.colour})
	}
	if card.mark {
		start := len(lead.text) + 1
		lead.text += " " + glyphs.operator
		lead.spans = append(lead.spans, colourSpan{start: start, end: start + len(glyphs.operator), colour: screen.Yellow})
	}
	with := func(slots ...string) drawnLine {
		line := drawnLine{text: lead.text, spans: lead.spans}
		for _, slot := range slots {
			if slot == "" {
				continue
			}
			gap := cell{text: line.text, width: displayWidth(line.text) + boardSlotGap}
			line.text = formatRow(row{cells: []cell{gap}, tail: slot}, 0)
		}
		return line
	}
	for _, form := range []drawnLine{with(card.holder, card.priority), with(card.holder)} {
		if displayWidth(form.text) <= width {
			return form
		}
	}
	room := width - displayWidth(lead.text) - boardSlotGap
	if card.holder != "" && room >= boardSlotMinimum {
		return with(cutText(card.holder, room, glyphs.ellipsis))
	}
	return cutLine(lead, width, glyphs.ellipsis)
}

// boardTitleLine is a card's second line: its title under the first line,
// indented by two or by what the column leaves room for, and cut to fit. A
// column is at least one character wide, since the window is at least two,
// so the indent is never negative.
func boardTitleLine(title string, width int, glyphs boardGlyphs) drawnLine {
	indent := boardTitleIndent
	if width-1 < indent {
		indent = width - 1
	}
	cut := strings.TrimRight(cutText(title, width-indent, glyphs.ellipsis), " ")
	if cut == "" {
		return drawnLine{}
	}
	return drawnLine{text: formatRow(row{indent: indent, tail: cut}, 0)}
}

// boardHeadingLine is the line a drawn view opens with at draw display
// columns: the view's title, and the acting clause ending in the last column
// drawn. Where the two do not fit with a gutter between them the line is the
// title alone, cut to fit.
func boardHeadingLine(title, acting string, draw int, ellipsis string) string {
	room := draw - displayWidth(acting)
	if acting == "" || room < displayWidth(title)+tableGutter {
		return cutText(title, draw, ellipsis)
	}
	return formatRow(row{cells: []cell{{text: title, width: room}}, tail: acting}, 0)
}

// boardProse lays a line of prose out at an indent in draw display columns:
// broken between words where draw is at least minTailColumns, which is the
// narrowest window the word breaker serves, and cut to fit otherwise. Every
// line it returns is cut to draw, so a word wider than the window cannot
// overrun it.
func boardProse(text string, indent, draw int, ellipsis string) []string {
	laid := formatRow(row{indent: indent, tail: text}, 0)
	if draw >= minTailColumns {
		laid = formatRow(row{indent: indent, tail: text, wrapTail: true}, draw)
	}
	var lines []string
	for _, line := range splitLines(laid) {
		lines = append(lines, cutText(line, draw, ellipsis))
	}
	return lines
}

// statusLine joins a watch's status parts with the separator and cuts them
// to room in the order the specification fixes: the change first, down to
// one unit and the ellipsis; then the time and its separator are dropped;
// then what remains is cut. The instruction to press Ctrl+C is therefore the
// last thing to go. An empty change is left out with its separator.
func statusLine(clock, change, stops, separator string, room int, ellipsis string) string {
	join := func(parts ...string) string {
		var kept []string
		for _, part := range parts {
			if part != "" {
				kept = append(kept, part)
			}
		}
		return strings.Join(kept, separator)
	}
	whole := join(clock, change, stops)
	if displayWidth(whole) <= room {
		return whole
	}
	if change != "" {
		first := textwidth.Cut(change, 1)
		if first == "" {
			first = textwidth.Cut(change, 2)
		}
		shortest := first + ellipsis
		changeRoom := room - displayWidth(join(clock, "x", stops)) + 1
		if changeRoom >= displayWidth(shortest) {
			return join(clock, cutText(change, changeRoom, ellipsis), stops)
		}
		change = shortest
	}
	remaining := join(change, stops)
	return cutText(remaining, room, ellipsis)
}
