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

// ruleGlyph is what a separator is drawn with. The table draws it rather than
// serving it from a catalog, so no translation can change it and no
// translation can misalign it.
const ruleGlyph = '-'

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
	laid := s.layOut(t)
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
}

// layOut removes the columns no row fills, chooses every column's width, and
// returns what the heading, separator and rows are all drawn from.
//
// The three passes run in one order and the order is load-bearing. The measure
// comes first, the near-miss widening second, and the narrow-window backstop
// last, so that the backstop has the final word on how much of the window the
// columns before the last one may take. Nothing widens a column after it. An
// earlier reading of this ran the widening a second time at the end and priced
// it at one display column, which is what one step costs rather than what a
// loop over every value costs: each value between a narrowed column's floor
// and the width it was measured at is a step the widening climbs back, so ten
// ordinary state names at a forty-column window walked three columns out again
// and put the last column past the room left for it.
func (s *session) layOut(t table) laidTable {
	window := s.width
	if window <= 0 {
		window = assumedWindow
	}
	laid := measure(t, window)
	narrowToWindow(&laid)
	return laid
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
		// silently put a labelInTheStack table's heading row back.
		labels:  t.labels,
		window:  window,
		columns: filled.columns,
		rows:    filled.rows,
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
		narrowed.rows = append(narrowed.rows, tableRow{section: r.section, fields: fields, note: r.note})
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
		rows = append(rows, tableRow{section: r.section, fields: r.fields[:end], note: r.note})
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
		dropped := fieldsOverWindow(r.fields, laid.indent, laid.window)
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
func fieldsOverWindow(fields []string, indent, window int) []bool {
	dropped := make([]bool, len(fields))
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
			total += displayWidth(field)
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
	if measured := laid.widths[len(laid.widths)-1]; measured < minTailColumns {
		return measured
	}
	return minTailColumns
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
func (laid laidTable) stacks() bool {
	if len(laid.columns) < 2 {
		return false
	}
	for _, r := range laid.rows {
		for c, field := range r.fields {
			if c == len(r.fields)-1 {
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
// the gutter after it, and the last field taking whatever is left of the line.
func (laid laidTable) rowLine(r tableRow) string {
	built := row{indent: laid.indent}
	for c, field := range r.fields {
		if c == len(r.fields)-1 {
			built.tail = field
			break
		}
		built.cells = append(built.cells, cell{text: field, width: laid.widths[c] + tableGutter})
	}
	return formatRow(built, laid.window)
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
