package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/msg"
)

// The three titles the sweep files its cards under. Each is a script whose
// drawn width disagrees with its rune count in a different way, and each
// reaches every block that renders a card title.
const (
	// wideTitle draws twice its rune count, since every rune is East Asian
	// Wide.
	wideTitle = "作業台のカード"
	// matraTitle carries spacing vowel signs, which take a column each, and
	// nonspacing ones, which take none, so a rune count measures it long.
	matraTitle = "हिन्दी कार्ड"
	// joinedTitle carries an emoji sequence a terminal draws as one glyph and
	// a rune count measures as three.
	joinedTitle = "\U0001F468\u200D\U0001F469\u200D\U0001F467 family"
)

// sweptBlock is one entry of the spec's inventory: what it draws, the columns
// it declares by catalog key, and how a fixture reaches it.
//
// Entries and call sites are many to one. Every entry names a site that
// exists, several entries may name one site, and every site the AST walk finds
// is named by at least one entry, which TestEveryTableSiteIsRegistered
// asserts in both directions. One tableLines call in formatCandidateRows draws
// the workbench listing on stdout and the ambiguous-workbench refusal on
// stderr, and renderOffers draws a state that offers a card and a state that
// offers nothing out of one table.
type sweptBlock struct {
	// site is the file and line of the s.table or s.tableLines call this
	// block is drawn through, relative to cmd/dinah.
	site string
	// label names the block for a failure message.
	label string
	// keys are the catalog keys of the block's headings, in column order. An
	// empty slice is a block of one column, which prints a list under a
	// sentence that already names it and takes neither a heading nor a
	// separator.
	keys []string
	// varies is the column whose display width the fixture makes differ
	// between two rows, which is what stops the assertion about the last
	// column holding whatever the measure does. It defaults to the column in
	// front of the last one. A negative value declares that no column of this
	// block varies, and constantReason says why.
	varies int
	// constantReason is why this block's fields take values no command
	// varies. It is empty on every block whose columns vary.
	constantReason string
	// render runs the command that draws this block and returns its lines.
	render func(t *testing.T, w *sweptWorkbenches, tag string) []string
	// shape is an extra assertion about the rows this entry exists for, on a
	// block two entries share. It is nil on every block whose entry asserts
	// nothing beyond the six below.
	shape func(t *testing.T, tag string, rows [][]string)
}

// sweptWindow is the window every block below is measured against. The suite
// clears COLUMNS before any test runs, so no documented source states a width
// and the table reads that as eighty columns.
const sweptWindow = 80

// sweptGutter is the two display columns that separate one column from the
// next, which is the table's own constant read from the output side.
const sweptGutter = 2

// sweptWorkbenches are the four trees the sweep renders from. Thirteen sites
// draw from a workbench holding a few cards and nothing unusual. The other
// seven draw only in a state a healthy workbench never reaches, so each of
// those gets a tree built for it.
type sweptWorkbenches struct {
	// healthy holds three cards under three scripts, one held, one blocked,
	// one carrying a link and a comment, with an operator-owned state and a
	// state under a limit.
	healthy string
	// base is where a tree a repair mutates is built. A migration repairs the
	// workbench it is run against, so the three blocks reached by one cannot
	// share a tree across the language pass and each builds its own.
	base string
	// ambiguous holds two workbenches under one base, which is the search
	// that resolves to a choice rather than to a workbench.
	ambiguous string
	// card is a reference the healthy tree carries.
	card string
	// held is the reference of the card claimed in the healthy tree.
	held string
}

// TestEveryRowStartsItsColumnsAtOneDisplayColumn renders every block of the
// spec's inventory in every shipped language and asserts six things about what
// came out, per block and per language:
//
//  1. The first line is a heading row carrying the catalog's text for each
//     declared column, and the second line is the separator row.
//  2. Every field of every row starts at the display column its own heading
//     starts at, so the headings and the rows are read against each other
//     rather than against a number typed into this test.
//  3. Every rule of the separator starts where its heading starts and is
//     exactly as wide, in display columns, as the column it sits under, with
//     the gutter left blank between neighbouring rules and no rule running
//     past the right edge of the window.
//  4. No two fields touch: every column is followed by at least the gutter.
//  5. Every column is tight, meaning no column is wider than the widest thing
//     in it and none is narrower than its own heading. This is what fails when
//     a width is declared rather than measured.
//  6. No line ends in a space.
//
// It is one of the backstops for what the source guard cannot see. A row
// padded with a byte length, a row padded by filling a slice of bytes with
// spaces, and a row composed inside a message catalog all reach a person as
// output, and these assertions hold whatever produced the line.
//
// Four self-checks keep the sweep from covering less than it claims. It fails
// when a site emitted no row at all, which is what leaves a whole block
// untested while the suite reports green. It fails when a block emitted fewer
// than two rows, since one row cannot disagree with itself. It requires the
// column in front of the last one to differ in display width between two rows,
// since a measured column takes the widest field and a block whose fields
// never vary cannot show that the measure was right. And a block whose rows
// all fit the window is asserted to draw no continuation line at all, which is
// what fails when a column is narrower than the rows it holds.
func TestEveryRowStartsItsColumnsAtOneDisplayColumn(t *testing.T) {
	benches := buildSweptWorkbenches(t)
	for _, block := range sweptBlocks() {
		rendered := 0
		for _, tag := range msg.Tags() {
			lines := block.render(t, benches, tag)
			if len(lines) == 0 {
				t.Errorf("%s (%s), locale %s: the fixture reached no row of this block, so nothing about it is asserted", block.site, block.label, tag)
				continue
			}
			rendered++
			assertNoLineEndsInASpace(t, block, tag, lines)
			if len(block.keys) == 0 {
				assertNoHeadingIsDrawn(t, block, tag, lines)
				continue
			}
			columns := assertHeadingRow(t, block, tag, lines)
			if columns == nil {
				continue
			}
			rows := readSweptRows(t, block, tag, lines[2:], columns)
			if len(rows) < 2 {
				t.Errorf("%s (%s), locale %s: the block rendered %d row, and one row cannot disagree with itself", block.site, block.label, tag, len(rows))
				continue
			}
			assertSeparatorRow(t, block, tag, lines[1], columns, rows)
			assertEveryColumnIsTight(t, block, tag, columns, rows)
			assertACellVaries(t, block, tag, rows)
			assertNoRowThatFitsIsContinued(t, block, tag, lines, columns, rows)
			if block.shape != nil {
				block.shape(t, tag, rows)
			}
		}
		if rendered == 0 {
			t.Errorf("%s (%s) rendered in no locale at all", block.site, block.label)
		}
	}
}

// assertHeadingRow asserts that the block opens with a heading row carrying
// the catalog's text for each declared column, and returns the display column
// each column begins at. It returns nil when the heading row is not what the
// block declares, since every assertion after it reads those columns.
func assertHeadingRow(t *testing.T, block sweptBlock, tag string, lines []string) []int {
	t.Helper()
	if len(lines) < 3 {
		t.Errorf("%s (%s), locale %s: the block drew %d lines, and a table draws a heading, a separator and its rows", block.site, block.label, tag, len(lines))
		return nil
	}
	heading := lines[0]
	columns := make([]int, 0, len(block.keys))
	cursor := 0
	for _, key := range block.keys {
		text := msg.For(tag).T(key)
		at := strings.Index(heading[cursor:], text)
		if at < 0 {
			t.Errorf("%s (%s), locale %s: the heading row carries no field for %s:\n%q", block.site, block.label, tag, key, heading)
			return nil
		}
		columns = append(columns, displayWidth(heading[:cursor+at]))
		cursor += at + len(text)
	}
	if columns[0] != sweptIndent {
		t.Errorf("%s (%s), locale %s: the heading row begins at display column %d and every block of this inventory is indented to %d:\n%q",
			block.site, block.label, tag, columns[0], sweptIndent, heading)
		return nil
	}
	for i := 1; i < len(columns); i++ {
		heldBefore := displayWidth(msg.For(tag).T(block.keys[i-1]))
		if columns[i]-columns[i-1] < heldBefore+sweptGutter {
			t.Errorf("%s (%s), locale %s: the heading %s begins at display column %d, which leaves less than the gutter after the heading before it:\n%q",
				block.site, block.label, tag, block.keys[i], columns[i], heading)
			return nil
		}
	}
	return columns
}

// assertNoHeadingIsDrawn asserts that a block of one column prints neither a
// heading row nor a separator row, since one column under a sentence that
// already names it is a list rather than a table.
func assertNoHeadingIsDrawn(t *testing.T, block sweptBlock, tag string, lines []string) {
	t.Helper()
	for _, line := range lines {
		if strings.Trim(strings.TrimSpace(line), "-") == "" && strings.Contains(line, "-") {
			t.Errorf("%s (%s), locale %s: a block of one column drew a separator row:\n%q", block.site, block.label, tag, line)
		}
	}
}

// assertSeparatorRow asserts the third of the sweep's six: one rule per
// column, each starting where its own heading starts and drawn exactly as wide
// as the column beneath it, with the gutter between neighbours and nothing
// past the right edge of the window.
//
// The last column's rule is measured the same way every other column's is,
// from the longest of its heading and its own values, which is the width that
// column carries for the separator alone. Every rule is read in display
// columns rather than in characters, because a rule counted in characters
// agrees with one counted in screen columns until the first Devanagari or CJK
// value arrives.
func assertSeparatorRow(t *testing.T, block sweptBlock, tag, separator string, columns []int, rows [][]string) {
	t.Helper()
	rules := sweptRules(separator)
	if len(rules) != len(columns) {
		t.Errorf("%s (%s), locale %s: the separator draws %d rules under %d columns:\n%q",
			block.site, block.label, tag, len(rules), len(columns), separator)
		return
	}
	last := len(columns) - 1
	for i, drawn := range rules {
		if drawn.at != columns[i] {
			t.Errorf("%s (%s), locale %s: the rule under %s begins at display column %d and its heading begins at %d:\n%q",
				block.site, block.label, tag, block.keys[i], drawn.at, columns[i], separator)
			continue
		}
		want := 0
		if i < last {
			want = columns[i+1] - columns[i] - sweptGutter
		} else {
			want = displayWidth(msg.For(tag).T(block.keys[i]))
			for _, row := range rows {
				if i < len(row) && displayWidth(row[i]) > want {
					want = displayWidth(row[i])
				}
			}
		}
		if room := sweptWindow - columns[i]; room < want {
			want = room
		}
		if want < 1 {
			want = 1
		}
		if drawn.width != want {
			t.Errorf("%s (%s), locale %s: the rule under %s draws %d display columns and its column is %d wide:\n%q",
				block.site, block.label, tag, block.keys[i], drawn.width, want, separator)
		}
	}
}

// sweptCarriesAFieldAt reports whether a line begins a field at any of the
// display columns given, meaning a non-space with the whole gutter blank in
// front of it. It is what separates a row that stops short, whose last field
// spans the rest of the line and is followed by nothing, from a row whose
// fields have run together, where a second field really does begin further
// along the same line.
func sweptCarriesAFieldAt(line string, columns []int) bool {
	for _, at := range columns {
		if displayWidth(line) <= at || sweptSpaceAt(line, at) {
			continue
		}
		if sweptBlank(line, at-sweptGutter, at) {
			return true
		}
	}
	return false
}

// sweptRule is one rule of a separator row: where it begins and how wide it
// draws, both in display columns.
type sweptRule struct {
	at    int
	width int
}

// sweptRules reads a separator row back into the rules it draws, which are the
// runs of the rule glyph separated by the blank gutter.
func sweptRules(separator string) []sweptRule {
	var rules []sweptRule
	column := 0
	open := false
	for _, r := range separator {
		if r == '-' {
			if !open {
				rules = append(rules, sweptRule{at: column})
				open = true
			}
			rules[len(rules)-1].width += displayWidth(string(r))
		} else {
			open = false
		}
		column += displayWidth(string(r))
	}
	return rules
}

// assertEveryColumnIsTight asserts the fifth of the sweep's six. A column is
// never wider than the widest thing in it, heading included, which is what a
// width declared at the call site produces and what a measured width cannot.
// It is never narrower than its own heading either, since a heading is a
// floor.
//
// A column narrower than one of its values is allowed and is not a departure
// from the measure: a field the window rules out of the measurement and a
// column the narrow-window backstop has taken down both land there, and the
// row renderer gives such a field the rest of its own line, which the fold
// above has already read.
func assertEveryColumnIsTight(t *testing.T, block sweptBlock, tag string, columns []int, rows [][]string) {
	t.Helper()
	for i := 0; i < len(columns)-1; i++ {
		width := columns[i+1] - columns[i] - sweptGutter
		heading := displayWidth(msg.For(tag).T(block.keys[i]))
		widest := heading
		for _, row := range rows {
			if i < len(row) && displayWidth(row[i]) > widest {
				widest = displayWidth(row[i])
			}
		}
		if width < heading {
			t.Errorf("%s (%s), locale %s: the column under %s is %d wide and its own heading draws %d",
				block.site, block.label, tag, block.keys[i], width, heading)
		}
		if width > widest {
			t.Errorf("%s (%s), locale %s: the column under %s is %d wide and the widest thing in it draws %d, so the width was declared rather than measured",
				block.site, block.label, tag, block.keys[i], width, widest)
		}
	}
}

// assertNoLineEndsInASpace asserts the sixth of the sweep's six. A trailing
// run of spaces is invisible on screen and shows up in a diff, in a pasted
// transcript and in a defect report, which is where it costs somebody time.
func assertNoLineEndsInASpace(t *testing.T, block sweptBlock, tag string, lines []string) {
	t.Helper()
	for _, line := range lines {
		if strings.HasSuffix(line, " ") {
			t.Errorf("%s (%s), locale %s: a line ends in a space:\n%q", block.site, block.label, tag, line)
		}
	}
}

// assertNoRowThatFitsIsContinued asserts that a block whose every row can be
// laid out inside the window draws one line per row, which is what fails when
// a column is narrower than the rows it holds.
//
// Two shapes sit outside it and say so rather than being quietly excused. A
// block holding a row the window cannot take however the columns are chosen
// draws that row over two lines by design, and so does a block holding a
// field wider than the column it sits in, which is what a field the window
// ruled out of the measurement and a column the narrow-window backstop took
// down both leave behind.
func assertNoRowThatFitsIsContinued(t *testing.T, block sweptBlock, tag string, lines []string, columns []int, rows [][]string) {
	t.Helper()
	for _, row := range rows {
		packed := sweptIndent
		for i, field := range row {
			if i > 0 {
				packed += sweptGutter
			}
			packed += displayWidth(field)
		}
		if packed > sweptWindow {
			return
		}
	}
	for i := 0; i < len(columns)-1; i++ {
		width := columns[i+1] - columns[i] - sweptGutter
		for _, row := range rows {
			if i < len(row) && displayWidth(row[i]) > width {
				return
			}
		}
	}
	if len(lines) != len(rows)+2 {
		t.Errorf("%s (%s), locale %s: every row of this block fits the window and the block drew %d lines under its heading for %d rows",
			block.site, block.label, tag, len(lines)-2, len(rows))
	}
}

// sweptIndent is the display column every block of this inventory starts at.
const sweptIndent = 2

// readSweptRows folds a block's rendered rows back into fields and asserts,
// for every field of every row, that it begins at the display column its own
// heading begins at. It returns each row's fields.
//
// The walk mirrors what formatRow does. A field under its column is padded to
// it and the gutter, so the next field begins at its heading's column. A field
// that reaches its column takes the rest of its own line, and the fields after
// it resume on the next line at the column the field's own would have ended
// in. A row may also stop short, which is how a state offering nothing says so
// where the card reference would have been: the field it stops on takes the
// rest of the line and the row ends there. Anything else is a row whose fields
// have drifted, which is what this test exists to catch.
func readSweptRows(t *testing.T, block sweptBlock, tag string, lines []string, columns []int) [][]string {
	t.Helper()
	var rows [][]string
	var row []string
	next := 0
	fail := func(format string, args ...any) [][]string {
		t.Helper()
		t.Errorf("%s (%s), locale %s: "+format, append([]any{block.site, block.label, tag}, args...)...)
		return rows
	}
	closeRow := func() {
		rows, row, next = append(rows, row), nil, 0
	}
	for i, line := range lines {
		if sweptLead(line) != columns[next] {
			return fail("field %d begins at display column %d and its heading begins at %d:\n%q",
				next, sweptLead(line), columns[next], line)
		}
		for {
			if next == len(columns)-1 {
				row = append(row, sweptField(line, columns[next], -1))
				closeRow()
				break
			}
			edge := columns[next+1]
			resumes := i+1 < len(lines) && sweptLead(lines[i+1]) == edge
			if displayWidth(line) <= edge {
				row = append(row, sweptField(line, columns[next], -1))
				next++
				if resumes {
					break
				}
				closeRow()
				break
			}
			if sweptBlank(line, columns[next], edge) {
				row = append(row, "")
				next++
				continue
			}
			if sweptSpaceAt(line, columns[next]) {
				return fail("field %d begins past the display column %d its heading begins at:\n%q", next, columns[next], line)
			}
			if !sweptBlank(line, edge-sweptGutter, edge) {
				if resumes {
					row = append(row, sweptField(line, columns[next], -1))
					next++
					break
				}
				if sweptCarriesAFieldAt(line, columns[next+1:]) {
					return fail("field %d runs into the gutter before display column %d, where field %d begins:\n%q",
						next, edge, next+1, line)
				}
				row = append(row, sweptField(line, columns[next], -1))
				next++
				closeRow()
				break
			}
			row = append(row, sweptField(line, columns[next], edge))
			next++
		}
	}
	if next != 0 {
		return fail("a row ran out of lines with field %d still to come", next)
	}
	return rows
}

// sweptBlank reports whether every display column between two boundaries holds
// a space, which is what a field carrying no text at all leaves behind. A
// column past the line's end holds nothing and is not a space, so a line that
// stops before the boundary is not blank to it.
func sweptBlank(line string, from, to int) bool {
	for column := from; column < to; column++ {
		if !sweptSpaceAt(line, column) {
			return false
		}
	}
	return true
}

// sweptLead reports the display column a line's content begins at.
func sweptLead(line string) int {
	return displayWidth(line) - displayWidth(strings.TrimLeft(line, " "))
}

// sweptSpaceAt reports whether the display column at holds a space. A column
// past the line's end holds nothing and is not a space.
//
// A rune drawing no column of its own belongs to the column its base
// character opened and never occupies the next one. Devanagari writes half its
// vowels that way, so a walk that let a nonspacing mark answer for the column
// after the word would read the padding behind a Hindi field as text and
// report a block that lines up as one that does not.
func sweptSpaceAt(line string, at int) bool {
	column := 0
	for _, r := range line {
		drawn := displayWidth(string(r))
		if drawn == 0 {
			continue
		}
		if column == at {
			return r == ' '
		}
		column += drawn
		if column > at {
			return false
		}
	}
	return false
}

// sweptField returns the text between two display columns, trimmed of the
// padding that carried it to the next one. An end of minus one takes the rest
// of the line.
func sweptField(line string, from, to int) string {
	var b strings.Builder
	column := 0
	inside := false
	for _, r := range line {
		drawn := displayWidth(string(r))
		if drawn > 0 {
			inside = column >= from && (to < 0 || column < to)
		}
		if inside {
			b.WriteRune(r)
		}
		column += drawn
	}
	return strings.TrimRight(b.String(), " ")
}

// assertACellVaries asserts that the column a block declares as its varying
// one really does differ in display width between two of the block's rows.
// Without it the column after it begins in the same place on every row
// whatever the measure does, and asserting where it begins proves nothing.
func assertACellVaries(t *testing.T, block sweptBlock, tag string, rows [][]string) {
	t.Helper()
	if len(block.keys) == 0 || block.varies == noCell {
		if block.varies == noCell && block.constantReason == "" {
			t.Errorf("%s (%s) declares that no column varies and gives no reason", block.site, block.label)
		}
		return
	}
	field := block.varies
	if field == lastCell {
		field = len(block.keys) - 2
	}
	if field < 0 {
		return
	}
	widths := map[int]bool{}
	for _, row := range rows {
		if field < len(row) {
			widths[displayWidth(row[field])] = true
		}
	}
	if len(widths) < 2 {
		t.Errorf("%s (%s), locale %s: column %d draws the same width on every row, so the column after it begins in the same place whatever the measure does",
			block.site, block.label, tag, field)
	}
}

// indentedBlock returns the indented lines of an output that follow a heading,
// stopping at the first line that is neither indented nor blank. A heading of
// the empty string starts at the top of the output.
func indentedBlock(out, heading string) []string {
	var lines []string
	started := heading == ""
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if !started {
			started = line == heading
			continue
		}
		if strings.TrimSpace(line) == "" {
			if len(lines) > 0 {
				break
			}
			continue
		}
		if !strings.HasPrefix(line, "  ") {
			if len(lines) > 0 {
				break
			}
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

// indentedLinesAfter returns every indented line following a heading, over any
// number of unindented lines between them. A comment's body sits unindented
// between two comment headers, so the comment block cannot stop at the first
// line that is not indented.
func indentedLinesAfter(out, heading string) []string {
	var lines []string
	started := false
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if !started {
			started = line == heading
			continue
		}
		if strings.HasPrefix(line, "  ") && strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// The two values sweptBlock.varies takes beyond a column index.
const (
	// lastCell asks for the column in front of the last one, which is the one
	// whose width decides where the last column begins.
	lastCell = -2
	// noCell declares that no column of a block varies, which a block may only
	// do with a reason.
	noCell = -1
)

// sweptBlocks are the twenty-one entries of the spec's inventory, at the
// nineteen call sites they render through.
func sweptBlocks() []sweptBlock {
	rendered := func(tag, key string) string { return msg.For(tag).T(key) }
	return []sweptBlock{
		{
			site: "render.go:128", label: "the legal moves under a served instruction",
			keys: []string{"column.moves.state", "column.moves.name", "column.moves.direction"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "instructions", w.card)
				return indentedBlock(out, rendered(tag, "instructions.moves"))
			},
		},
		{
			site: "render.go:148", label: "the cards you hold",
			keys: []string{"column.holding.card", "column.holding.title"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "status")
				return indentedBlock(out, rendered(tag, "status.holding"))
			},
		},
		{
			site: "render.go:157", label: "the cards that are blocked",
			keys: []string{"column.blocked.card", "column.blocked.reason"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "status")
				return indentedBlock(out, rendered(tag, "status.blocked"))
			},
		},
		{
			site: "render.go:184", label: "dinah states",
			keys:   []string{"column.states.slug", "column.states.name", "column.states.kind", "column.states.cards", "column.states.owner"},
			varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.healthy, tag, "states"), "")
			},
		},
		{
			site: "render.go:197", label: "dinah ls",
			keys: []string{"column.ls.card", "column.ls.standing", "column.ls.title"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.healthy, tag, "ls"), "")
			},
		},
		{
			site: "render.go:209", label: "dinah config",
			keys: []string{"column.config.setting", "column.config.value", "column.config.source"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.healthy, tag, "config"), "")
			},
		},
		{
			site: "render.go:237", label: "dinah workbenches",
			keys: []string{"column.workbenches.workbench", "column.workbenches.slug", "column.workbenches.path"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.ambiguous, tag, "workbenches"), "")
			},
		},
		{
			site: "render.go:237", label: "the ambiguous-workbench refusal, written to stderr",
			keys: []string{"column.workbenches.workbench", "column.workbenches.slug", "column.workbenches.path"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRefused(t, w.ambiguous, tag, "status"), "")
			},
		},
		{
			site: "render.go:263", label: "dinah next, a state offering a card",
			keys: []string{"column.next.state", "column.next.card", "column.next.title"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.healthy, tag, "next"), "")
			},
			shape: func(t *testing.T, tag string, rows [][]string) {
				t.Helper()
				for _, row := range rows {
					if len(row) == 3 && row[1] != "" {
						return
					}
				}
				t.Errorf("dinah next drew no state offering a card in %s, so the shape this entry exists for was not rendered", tag)
			},
		},
		{
			site: "render.go:263", label: "dinah next, a state offering nothing",
			keys: []string{"column.next.state", "column.next.card", "column.next.title"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.healthy, tag, "next"), "")
			},
			shape: func(t *testing.T, tag string, rows [][]string) {
				t.Helper()
				none := msg.For(tag).T("next.none")
				for _, row := range rows {
					if len(row) == 2 && row[1] == none {
						return
					}
				}
				t.Errorf("dinah next drew no state offering nothing in %s, so the row that stops short was not rendered", tag)
			},
		},
		{
			site: "render.go:280", label: "a card's links",
			keys: []string{"column.links.link", "column.links.card"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "show", w.card)
				return indentedBlock(out, rendered(tag, "show.links"))
			},
		},
		{
			site: "render.go:290", label: "a card's comments",
			keys: []string{"column.comments.when", "column.comments.who"}, varies: noCell,
			constantReason: "a timestamp is one format in one time zone, so every comment header draws its stamp " +
				"at the same width; the author in the last column is what this block's assertion rests on",
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "show", w.card)
				return indentedLinesAfter(out, rendered(tag, "show.comments"))
			},
		},
		{
			site: "render.go:315", label: "dinah log",
			keys: []string{"column.log.when", "column.log.action", "column.log.actor", "column.log.detail"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.healthy, tag, "log", w.held), "")
			},
		},
		{
			site: "render.go:331", label: "the slugs check --migrate-slugs assigned",
			keys: []string{"column.slugs.slug", "column.slugs.title"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, sweptStrippedTree(t, w, "slugs-"+tag), tag, "check", "--migrate-slugs"), "")
			},
		},
		{
			site: "render.go:345", label: "one removed stranded state", varies: noCell,
			constantReason: "this block declares one column and no heading, so it has no column to misplace",
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, sweptStrandedTree(t, w, "stranded-"+tag), tag, "check", "--migrate-states"), "")
			},
		},
		{
			site: "render.go:362", label: "one finding", varies: noCell,
			constantReason: "this block declares one column and no heading, so it has no column to misplace",
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRefused(t, sweptStrippedTree(t, w, "findings-"+tag), tag, "check"), "")
			},
		},
		{
			site: "render.go:387", label: "catalog coverage",
			keys: []string{"column.catalogs.language", "column.catalogs.translated"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "version", "--catalogs")
				return indentedBlock(out, rendered(tag, "version.catalogs"))
			},
		},
		{
			site: "help.go:101", label: "the command list of bare dinah",
			keys: []string{"column.commands.command", "column.commands.what"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag)
				return indentedBlock(out, rendered(tag, "help.group.work"))
			},
		},
		{
			site: "help.go:109", label: "the global flag list",
			keys: []string{"column.flags.option", "column.flags.what"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag)
				return indentedBlock(out, rendered(tag, "help.flags"))
			},
		},
		{
			site: "help.go:142", label: "dinah help <command>",
			keys: []string{"column.help.order", "column.help.check", "column.help.refusal"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "help", "add")
				return indentedBlock(out, rendered(tag, "help.refusals"))
			},
		},
		{
			site: "commands.go:361", label: "the guide topics",
			keys: []string{"column.guide.topic", "column.guide.title"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.healthy, tag, "guide"), "")
			},
		},
	}
}

// sweptRun runs a command in a language and returns what it wrote to stdout,
// failing when the command refused.
func sweptRun(t *testing.T, dir, tag string, argv ...string) string {
	t.Helper()
	got := runCLI(t, dir, append([]string{"--lang", tag}, argv...)...)
	if got.code != 0 {
		t.Fatalf("%v in %s: exit %d\n%s", argv, tag, got.code, got.errw)
	}
	return got.out
}

// sweptRefused runs a command expected to refuse and returns what it wrote,
// stdout and stderr alike, since one of the two blocks reached this way is
// written to each.
func sweptRefused(t *testing.T, dir, tag string, argv ...string) string {
	t.Helper()
	got := runCLI(t, dir, append([]string{"--lang", tag}, argv...)...)
	if got.code == 0 {
		t.Fatalf("%v in %s was expected to refuse and exited 0\n%s", argv, tag, got.out)
	}
	return got.out + got.errw
}

// sweptTitles are the titles the sweep files its cards under, cycling so that
// every block rendering a card title meets each script.
var sweptTitles = []string{wideTitle, matraTitle, joinedTitle, "a plain card"}

// reviewState is the fourth state the sweep writes into the healthy workbench.
// The flow init builds has three, which leaves dinah next with one state
// offering a card and one offering nothing, and a block of one row cannot
// disagree with itself. This state is operator-owned as well, so dinah states
// draws its own tail on at least one row.
// waitingState is the fifth, which leaves two states offering nothing, since a
// block of one row cannot disagree with itself either.
const (
	reviewState  = "e00000000001"
	waitingState = "e00000000002"
)

// reviewTitle is written in a script drawing two columns per rune, so the
// title column of dinah states and of dinah next carries text a rune count
// measures short.
const reviewTitle = "検討"

// buildSweptWorkbenches builds the four trees the sweep renders from, under
// one user base so that one language pass reaches all of them.
func buildSweptWorkbenches(t *testing.T) *sweptWorkbenches {
	t.Helper()
	base := t.TempDir()
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	t.Setenv("DINAH_ACTOR", "alka")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_WORKBENCH", "")
	t.Setenv("COLUMNS", "")

	benches := &sweptWorkbenches{
		base:      base,
		healthy:   filepath.Join(base, "healthy"),
		ambiguous: filepath.Join(base, "ambiguous"),
		card:      "fx-1",
		held:      "fx-1",
	}
	for _, dir := range []string{benches.healthy, benches.ambiguous} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	sweptInit(t, benches.healthy)
	sweptAddState(t, benches.healthy, reviewState, reviewTitle, "work", "operator_owned: true\n")
	sweptAddState(t, benches.healthy, waitingState, "Waiting", "work", "")
	for i := 0; i < 12; i++ {
		sweptDo(t, benches.healthy, "add", sweptTitles[i%len(sweptTitles)])
	}
	sweptDo(t, benches.healthy, "claim", "fx-1")
	sweptDo(t, benches.healthy, "claim", "fx-11")
	sweptDo(t, benches.healthy, "claim", "fx-2", "--actor", "bo")
	sweptDo(t, benches.healthy, "block", "fx-2", "waiting on a decision", "--actor", "bo")
	sweptDo(t, benches.healthy, "claim", "fx-10", "--actor", "bo")
	sweptDo(t, benches.healthy, "block", "fx-10", "waiting on a decision", "--actor", "bo")
	sweptDo(t, benches.healthy, "claim", "fx-4")
	sweptDo(t, benches.healthy, "move", "fx-4", "doing")
	sweptDo(t, benches.healthy, "release", "fx-4")
	sweptDo(t, benches.healthy, "claim", "fx-12")
	sweptDo(t, benches.healthy, "move", "fx-12", "doing")
	sweptDo(t, benches.healthy, "move", "fx-12", "done")
	sweptDo(t, benches.healthy, "release", "fx-12")
	sweptDo(t, benches.healthy, "comment", "fx-1", "the first note")
	sweptDo(t, benches.healthy, "comment", "fx-1", "the second note", "--actor", "bo")
	sweptWriteLinks(t, benches.healthy, "fx-1")

	rooms := populateBase(t, filepath.Join(benches.ambiguous, bench.UserBaseName), "one", "twoandthree")
	sweptRetitle(t, rooms[0], wideTitle)
	sweptRetitle(t, rooms[1], matraTitle)

	return benches
}

// sweptStrippedTree builds a workbench whose states carry no slug, which is
// the defect a plain check reports one finding at a time and check
// --migrate-slugs repairs one row at a time. Each call builds its own, since
// the repair is what the block renders and a repaired workbench draws nothing
// the second time.
func sweptStrippedTree(t *testing.T, w *sweptWorkbenches, name string) string {
	t.Helper()
	dir := filepath.Join(w.base, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	sweptInit(t, dir)
	sweptAddState(t, dir, reviewState, "Review under an operator", "work", "")
	sweptStripSlugs(t, dir)
	return dir
}

// sweptStrandedTree builds a workbench naming a state its own directory does
// not hold, which is what check --migrate-states removes. Each call builds its
// own, for the reason sweptStrippedTree gives.
func sweptStrandedTree(t *testing.T, w *sweptWorkbenches, name string) string {
	t.Helper()
	dir := filepath.Join(w.base, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	sweptInit(t, dir)
	sweptStrandState(t, dir)
	return dir
}

// sweptInit creates a workbench in a directory the sweep made.
func sweptInit(t *testing.T, dir string) {
	t.Helper()
	sweptDo(t, dir, "init", "--slug", "fx", "--operator", "alka")
}

// sweptDo runs a command that has to succeed while the fixture is being built.
func sweptDo(t *testing.T, dir string, argv ...string) {
	t.Helper()
	if got := runCLI(t, dir, argv...); got.code != 0 {
		t.Fatalf("%v: exit %d\n%s", argv, got.code, got.errw)
	}
}

// sweptRoot returns the one workbench directory a container holds.
func sweptRoot(t *testing.T, dir string) string {
	t.Helper()
	return soleBenchDir(t, dir)
}

// sweptAddState writes a state into a workbench by hand and appends it to the
// states list, since no command creates one.
func sweptAddState(t *testing.T, dir, id, title, kind, extra string) {
	t.Helper()
	root := sweptRoot(t, dir)
	states := filepath.Join(root, bench.StatesDir, id)
	if err := os.MkdirAll(states, 0o755); err != nil {
		t.Fatalf("mkdir state: %v", err)
	}
	anchor := "---\ntitle: " + title + "\nkind: " + kind + "\n" + extra + "---\n"
	if err := os.WriteFile(filepath.Join(states, bench.StateAnchor), []byte(anchor), 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}
	sweptRewrite(t, filepath.Join(root, bench.WorkbenchAnchor), func(source string) string {
		return strings.Replace(source, "\nstates:\n", "\nstates:\n  - "+id+"\n", 1)
	})
}

// sweptStrandState names a state the workbench does not hold, which is the
// defect check --migrate-states removes.
func sweptStrandState(t *testing.T, dir string) {
	t.Helper()
	sweptRewrite(t, filepath.Join(sweptRoot(t, dir), bench.WorkbenchAnchor), func(source string) string {
		return strings.Replace(source, "\nstates:\n", "\nstates:\n  - ffffffffffff\n", 1)
	})
}

// sweptStripSlugs removes the slug line from every state, which is the defect
// a plain check reports and check --migrate-slugs repairs one row at a time.
func sweptStripSlugs(t *testing.T, dir string) {
	t.Helper()
	states := filepath.Join(sweptRoot(t, dir), bench.StatesDir)
	entries, err := os.ReadDir(states)
	if err != nil {
		t.Fatalf("read states: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sweptRewrite(t, filepath.Join(states, entry.Name(), bench.StateAnchor), func(source string) string {
			var kept []string
			for _, line := range strings.Split(source, "\n") {
				if !strings.HasPrefix(line, "slug:") {
					kept = append(kept, line)
				}
			}
			return strings.Join(kept, "\n")
		})
	}
}

// sweptWriteLinks writes a links sequence into a card's frontmatter, since no
// command creates a link and the reader reads one.
func sweptWriteLinks(t *testing.T, dir, ref string) {
	t.Helper()
	located := runCLI(t, dir, "path", ref)
	if located.code != 0 {
		t.Fatalf("path %s: %d %s", ref, located.code, located.errw)
	}
	path := strings.TrimSpace(located.out)
	sweptRewrite(t, path, func(source string) string {
		links := "links:\n  - kind: blocks\n    to: fx-2\n  - kind: relates-to\n    to: fx-3\n"
		cut := strings.Index(source[4:], "\n---\n")
		return source[:cut+4] + "\n" + links + source[cut+5:]
	})
}

// sweptRetitle rewrites a workbench's own title, so two workbenches under one
// base draw titles of different widths.
func sweptRetitle(t *testing.T, root, title string) {
	t.Helper()
	sweptRewrite(t, filepath.Join(root, bench.WorkbenchAnchor), func(source string) string {
		var kept []string
		for _, line := range strings.Split(source, "\n") {
			if strings.HasPrefix(line, "title:") {
				line = "title: " + title
			}
			kept = append(kept, line)
		}
		return strings.Join(kept, "\n")
	})
}

// sweptRewrite reads a file, hands it to a change, and writes it back.
func sweptRewrite(t *testing.T, path string, change func(string) string) {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(change(string(source))), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
