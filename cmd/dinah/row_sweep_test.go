package main

import (
	"os"
	"path/filepath"
	"strconv"
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
	// wrapsTail says the block asks the renderer to break its last column
	// between words at the window, so a line leading at that column's own
	// display position continues the row above rather than opening a new one.
	wrapsTail bool
	// render runs the command that draws this block and returns its lines.
	render func(t *testing.T, w *sweptWorkbenches, tag string) []string
	// shape is an extra assertion about the rows this entry exists for, on a
	// block two entries share. It is nil on every block whose entry asserts
	// nothing beyond the six below.
	shape func(t *testing.T, tag string, rows [][]string)
	// blanksAreLost says the fixture harvests only the block's own indented
	// lines, so the blank line the stacked form draws between two records
	// never reaches this sweep. A block carrying a note has to be harvested
	// that way, since the note is printed at no indent between one record and
	// the next. The stacked assertions then read a label that does not advance
	// as the start of the next record rather than as a missing separator, and
	// everything else about the block is asserted as it is anywhere else.
	blanksAreLost bool
	// noHeadingRow says the call site declares labelInTheStack, so the columnar
	// form of this block draws neither a heading row nor a rule. The block
	// still declares its keys: its columns keep their headings, the measure
	// still floors each column at its own heading, and the stacked form still
	// labels every field with one. Every row-shape assertion runs against such
	// a block, with the column positions read from the rows instead of from a
	// heading row.
	noHeadingRow bool
}

// sweptWindows are the three windows every block below is drawn at: the one
// nobody stated, the eighty columns the table reads that as, and a window
// narrow enough to put the backstop and the clamp to work. A rule counted
// against the wrong edge is what the last of the three catches.
var sweptWindows = []struct {
	// columns is what COLUMNS is set to, empty for the window nobody stated.
	columns string
	// window is the width the table measures against, which is eighty when
	// no documented source states one.
	window int
	// full says every assertion runs on this pass. It is false on the narrow
	// pass, where the row renderer's own clamp moves a continuation line left
	// of the column it continues, by design and since dinah-101, so the rows
	// cannot be folded back against the headings there. What the narrow pass
	// asserts instead is the clamp this card adds: no rule past the right
	// edge, every rule under its own heading, and no line ending in a space.
	full bool
	// stacked says the window is narrow enough that a table gives up its shape
	// and draws each record as its own block. The pass fails when no block of
	// the corpus stacked, since a window that turns out to be too wide would
	// otherwise pass on an empty set. It is not asserted of every block: a
	// table of short fields under short headings holds its shape at any width,
	// and a block that keeps it is asserted as a table.
	stacked bool
}{
	{columns: "", window: 80, full: true},
	{columns: "80", window: 80, full: true},
	{columns: "40", window: 40},
	{columns: "20", window: 20, stacked: true},
}

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
// A block the window is too narrow for draws as a stack instead, one record to
// a block and one field to a line, and assertStackedBlock is what reads that
// form. The last pass draws the whole corpus at a window narrow enough to
// stack most of it and fails when none of it stacked; the workbench listing
// reaches the same assertions at eighty columns, where a long path takes its
// other two columns down to their headings.
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
// all fit the window, every field of them inside its own column, is asserted
// to draw no continuation line at all, which is what fails when an unpadded
// last field runs past the edge of a line that had room for every column.
func TestEveryRowStartsItsColumnsAtOneDisplayColumn(t *testing.T) {
	benches := buildSweptWorkbenches(t)
	for pass, drawn := range sweptWindows {
		t.Setenv("COLUMNS", drawn.columns)
		sweptWindow = drawn.window
		sweptPass = strconv.Itoa(pass)
		assertEveryBlockLinesUp(t, benches, drawn.full, drawn.stacked)
	}
}

// sweptWindow is the width the pass now running measures against, which every
// assertion below reads when it asks where the right edge is.
var sweptWindow = 80

// sweptPass names the pass now running, which goes into the name of every
// fixture tree a repair mutates. A migration repairs the workbench it runs
// against and draws nothing the second time, so each pass builds its own.
var sweptPass = "0"

// assertEveryBlockLinesUp runs the six assertions over every block in every
// shipped language, at whatever window the pass is drawing at.
//
// A block the window is too narrow for draws as a stack rather than as a
// table, and the stacked form has assertions of its own. That happens on every
// block of the narrow pass and on the workbench listing wherever a fixture's
// path is long enough, which at eighty columns it is. drawsTheStackedForm is
// what tells the two apart, and a block declaring noHeadingRow is held to that
// declaration first, since no reading of the output can catch a block drawing
// a heading row it said it would never draw.
func assertEveryBlockLinesUp(t *testing.T, benches *sweptWorkbenches, full, stacked bool) {
	t.Helper()
	stacks := 0
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
			if block.noHeadingRow && carriesTheHeadingRow(block, tag, lines[0]) {
				t.Errorf("%s (%s), locale %s: the block declares that it draws no heading row and drew one:\n%q", block.site, block.label, tag, lines[0])
				continue
			}
			if drawsTheStackedForm(block, tag, lines[0]) {
				assertStackedBlock(t, block, tag, lines)
				stacks++
				continue
			}
			columns, rowLines := sweptRowLines(t, block, tag, full, lines)
			if columns == nil {
				continue
			}
			rows := readSweptRows(t, block, tag, rowLines, columns)
			if len(rows) < 2 {
				t.Errorf("%s (%s), locale %s: the block rendered %d row, and one row cannot disagree with itself", block.site, block.label, tag, len(rows))
				continue
			}
			if !block.noHeadingRow {
				assertSeparatorRow(t, block, tag, lines[1], columns, rows)
			}
			assertEveryColumnIsTight(t, block, tag, columns, rows)
			assertACellVaries(t, block, tag, rows)
			assertNoRowThatFitsIsContinued(t, block, tag, rowLines, columns, rows)
			if block.shape != nil {
				block.shape(t, tag, rows)
			}
		}
		if rendered == 0 {
			t.Errorf("%s (%s) rendered in no locale at all", block.site, block.label)
		}
	}
	if !stacked {
		return
	}
	if stacks == 0 {
		t.Errorf("no block of the inventory stacked at a window of %d, so this pass asserts nothing about the stacked form", sweptWindow)
	}
	assertTheStackedCheckCanFail(t)
}

// assertTheStackedCheckCanFail arms the stacked assertions on a block of the
// pass's own, in every shipped language.
//
// Every stacked record the corpus draws carries a field under the widest
// heading its block declares, and where that is true a build padding each
// record to its own widest heading draws exactly what a correct one draws. So
// the corpus cannot tell the two apart, and a check that passes on both proves
// nothing. The block below is the one shape that separates them: its second
// record holds no standing, so a per-record padding puts that record's values
// three display columns left of the first record's, which is assertion four.
//
// It is rendered through the head's own tableLines and read by the same
// assertions the corpus is read by, so it arms them rather than standing in
// for them.
func assertTheStackedCheckCanFail(t *testing.T) {
	t.Helper()
	block := sweptBlock{
		site:  "row_sweep_test.go",
		label: "the control block the narrow pass arms itself with",
		keys:  []string{"column.ls.card", "column.ls.standing", "column.ls.title"},
	}
	for _, tag := range msg.Tags() {
		s := &session{r: msg.For(tag), width: sweptWindow}
		lines := s.tableLines(table{
			indent:  sweptIndent,
			columns: s.columns("ls", "card", "standing", "title"),
			rows: []tableRow{
				{fields: []string{"demo-1", msg.For(tag).T("token.ready"), "a card of some length"}},
				{fields: []string{"demo-2", "", "a second card of some length"}},
			},
		})
		if !drawsTheStackedForm(block, tag, lines[0]) {
			t.Errorf("locale %s: the control block held its shape at a window of %d, so it arms nothing:\n%q", tag, sweptWindow, lines[0])
			continue
		}
		assertStackedBlock(t, block, tag, lines)
	}
}

// drawsTheStackedForm reports whether a block drew the stacked form rather
// than a table, which decides which set of assertions the block is read by.
//
// It asks what the stacked form draws instead of what a table draws. Reading
// the absence of a heading row as a stack was safe only while every table drew
// one, and a call site declaring labelInTheStack draws a table with no heading
// row, which that reading calls a stack in every locale at every window.
//
// A stacked line opens with one of the block's own labels and carries a value
// behind it, and a table's row opens with a value. The heading row opens with
// a label too, so it is ruled out first, being the one line that carries every
// label of the block in column order.
func drawsTheStackedForm(block sweptBlock, tag string, line string) bool {
	if carriesTheHeadingRow(block, tag, line) {
		return false
	}
	return stackedLabel(line, sweptLabels(block, tag)) >= 0
}

// sweptLabels renders the block's headings in one language, in column order.
func sweptLabels(block sweptBlock, tag string) []string {
	labels := make([]string, 0, len(block.keys))
	for _, key := range block.keys {
		labels = append(labels, msg.For(tag).T(key))
	}
	return labels
}

// sweptRowLines asserts whatever this pass asks of the lines a block draws
// above its rows, and returns the display column each column begins at
// together with the lines that carry rows. It returns nil columns when the
// block has nothing further to assert on this pass, either because a failure
// has already been reported or because the narrow pass asks nothing more of
// it.
//
// The two forms differ in one line pair, and resolving that here is what keeps
// a count of skipped lines out of every assertion below. A block drawing a
// heading row takes its column positions out of that row and its rows out of
// the lines under the rule. A block declaring noHeadingRow draws no such pair,
// so its positions are read from its own rows and every line it drew is a row
// line.
//
// The narrow pass leaves a noHeadingRow block after assertNoHeadingIsDrawn and
// asserts nothing further about it. Both of the things that pass asks are
// about a line the block never prints, since assertNoRuleRunsPastTheEdge reads
// a rule and the derivation reads a row that fits on one line, which the row
// renderer's own clamp is free to deny it there.
func sweptRowLines(t *testing.T, block sweptBlock, tag string, full bool, lines []string) ([]int, []string) {
	t.Helper()
	if block.noHeadingRow {
		assertNoHeadingIsDrawn(t, block, tag, lines)
		if !full {
			return nil, nil
		}
		return deriveHeadinglessColumns(t, block, tag, lines), lines
	}
	columns := assertHeadingRow(t, block, tag, lines)
	if columns == nil {
		return nil, nil
	}
	if !full {
		assertNoRuleRunsPastTheEdge(t, block, tag, lines[1], columns)
		return nil, nil
	}
	return columns, lines[2:]
}

// deriveHeadinglessColumns reads the display column each column of a
// noHeadingRow block begins at out of the block's own rows, which is what the
// headed path reads out of its heading line.
//
// Every column but the last is padded to a fixed width by the one row
// renderer, so a run of at least the gutter's width of blank display columns
// behind a field marks where the next column begins. A line that begins
// somewhere other than the column under consideration is a continuation and is
// skipped before the scan.
//
// The rightmost candidate wins rather than the first or the leftmost. A field
// whose own value carries two adjacent spaces offers a false boundary to the
// left of the true one, since sweptNextFieldColumn stops at the first gap of
// the gutter's width, and the maximum discards that candidate where a
// first-wins rule adopts it. A genuine disagreement between rows is not
// settled by the maximum and is not asked to be: readSweptRows reads every row
// against the positions returned here and fails on any row whose fields begin
// somewhere else.
//
// Column 0 is taken at sweptIndent rather than asserted, for the same reason.
// A row indented anywhere else fails in readSweptRows.
//
// A column no row places calls t.Errorf naming the block, the locale and the
// column, then returns nil for the caller to read as a failure already
// reported, exactly as assertHeadingRow does. The loop runs until it holds one
// position per declared key, so a short slice never reaches the row checks. A
// block whose first column is empty on every row offers no boundary anywhere,
// since every line's lead sits where the second column begins, and it fails by
// that message rather than passing quietly.
func deriveHeadinglessColumns(t *testing.T, block sweptBlock, tag string, lines []string) []int {
	t.Helper()
	columns := []int{sweptIndent}
	for len(columns) < len(block.keys) {
		from := columns[len(columns)-1]
		at := -1
		for _, line := range lines {
			if sweptLead(line) != from {
				continue
			}
			if next := sweptNextFieldColumn(line, from); next > at {
				at = next
			}
		}
		if at < 0 {
			t.Errorf("%s (%s), locale %s: no row of this block shows where column %d begins, so nothing about its columns is asserted",
				block.site, block.label, tag, len(columns))
			return nil
		}
		columns = append(columns, at)
	}
	return columns
}

// sweptNextFieldColumn reports the display column the field after the one
// beginning at from starts in, or minus one when the line offers no boundary.
// The boundary is the first non-space that a run of at least the gutter's
// width of blank display columns stands in front of, which is what the row
// renderer leaves behind when it pads a field to its column. A line that stops
// short with no such gap after its last field offers no boundary.
func sweptNextFieldColumn(line string, from int) int {
	width := displayWidth(line)
	blank := 0
	for column := from; column < width; column++ {
		if sweptSpaceAt(line, column) {
			blank++
			continue
		}
		if blank >= sweptGutter {
			return column
		}
		blank = 0
	}
	return -1
}

// carriesTheHeadingRow reports whether a line is the heading row a block
// declares, which is what rules the heading row out of the stacked reading in
// drawsTheStackedForm and what holds a noHeadingRow block to its declaration.
// It reads the same catalog entries in the same order assertHeadingRow does
// and reports rather than failing, since either answer is a shape this sweep
// asserts against.
func carriesTheHeadingRow(block sweptBlock, tag, line string) bool {
	cursor := 0
	for _, key := range block.keys {
		text := msg.For(tag).T(key)
		at := strings.Index(line[cursor:], text)
		if at < 0 {
			return false
		}
		cursor += at + len(text)
	}
	return true
}

// assertStackedBlock asserts the six things the stacked form promises, per
// block and per language. It is the check the stacked form needs of its own:
// the output check that runs after every CLI invocation reads a stacked record
// as a two-column table, finds its values lined up, and can see nothing else
// about it.
//
//  1. The block draws neither a heading row nor a separator row.
//  2. Every line splits into a label and a value, where the label is the
//     catalog's text, in this language, for one of the block's declared
//     heading keys.
//  3. Each record's lines carry the block's columns in column order, one field
//     to a line, with a field holding no text absent.
//  4. Every value in the block begins at one display column, measured in
//     display columns against the widest heading the block declares, across
//     records as well as within one.
//  5. Exactly one blank line separates one record from the next, and the first
//     record draws none above it.
//  6. No line is a continuation, so the block draws exactly as many lines as
//     its records carry fields, plus the blank lines between them.
func assertStackedBlock(t *testing.T, block sweptBlock, tag string, lines []string) {
	t.Helper()
	fail := func(format string, args ...any) {
		t.Helper()
		t.Errorf("%s (%s), locale %s: "+format, append([]any{block.site, block.label, tag}, args...)...)
	}
	labels := make([]string, 0, len(block.keys))
	widest := 0
	for _, key := range block.keys {
		text := msg.For(tag).T(key)
		labels = append(labels, text)
		if drawn := displayWidth(text); drawn > widest {
			widest = drawn
		}
	}
	values := sweptIndent + widest + sweptGutter
	held, blank, records := -1, false, 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if records == 0 {
				fail("a blank line stands above the first record:\n%q", line)
				return
			}
			if blank {
				fail("two blank lines separate one record from the next")
				return
			}
			blank, held = true, -1
			continue
		}
		if carriesTheHeadingRow(block, tag, line) {
			fail("the stacked form drew a heading row:\n%q", line)
			return
		}
		if strings.Trim(strings.TrimSpace(line), string(ruleGlyph)) == "" {
			fail("the stacked form drew a separator row:\n%q", line)
			return
		}
		if lead := sweptLead(line); lead != sweptIndent {
			fail("a line begins at display column %d and every label of the block begins at %d, so the line is a continuation:\n%q", lead, sweptIndent, line)
			return
		}
		at := stackedLabel(line, labels)
		if at < 0 {
			fail("a line carries no label the block declares a heading for:\n%q", line)
			return
		}
		if at <= held {
			if !blank && !block.blanksAreLost {
				fail("the label %q follows %q with no blank line between the two records:\n%q", labels[at], labels[held], line)
				return
			}
			records++
		}
		if records == 0 {
			records++
		}
		held, blank = at, false
		if !sweptBlank(line, sweptIndent+displayWidth(labels[at]), values) {
			fail("the label %q is followed by something other than padding before display column %d:\n%q", labels[at], values, line)
			return
		}
		if sweptSpaceAt(line, values) || displayWidth(line) <= values {
			fail("the value after the label %q does not begin at display column %d, where the widest heading of the block leaves it:\n%q", labels[at], values, line)
			return
		}
	}
	if blank {
		fail("the block ends on a blank line, so a record was separated from nothing")
	}
	if records == 0 {
		fail("the block drew no record at all")
	}
}

// stackedLabel reports which of a block's headings a stacked line is labelled
// with, or minus one when it carries none. The longest match wins, since one
// heading can open another and a line labelled with the longer one would
// otherwise be read as the shorter.
func stackedLabel(line string, labels []string) int {
	text := strings.TrimLeft(line, " ")
	at, held := -1, 0
	for i, label := range labels {
		if !strings.HasPrefix(text, label) {
			continue
		}
		if drawn := displayWidth(label); at < 0 || drawn > held {
			at, held = i, drawn
		}
	}
	return at
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

// assertNoHeadingIsDrawn asserts that a block promising no heading row draws
// no separator row anywhere in it. Two kinds of block promise that. One column
// under a sentence that already names it is a list rather than a table, and a
// call site declaring labelInTheStack keeps its columns and draws the row of
// labels over them nowhere.
func assertNoHeadingIsDrawn(t *testing.T, block sweptBlock, tag string, lines []string) {
	t.Helper()
	for _, line := range lines {
		if strings.Trim(strings.TrimSpace(line), "-") == "" && strings.Contains(line, "-") {
			t.Errorf("%s (%s), locale %s: a headingless block drew a separator row:\n%q", block.site, block.label, tag, line)
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

// assertNoRuleRunsPastTheEdge is what the narrow pass asserts in place of the
// six: every rule sits under its own heading, none is wider than its column,
// none runs past the right edge of the window, and none is narrower than one
// column.
//
// The rows are not read here. A value wider than the column the backstop left
// it takes its own line, and the row renderer clamps the continuation under it
// so that the line keeps room for its own text, which moves the fields after
// it left of the columns they belong to. That is dinah-101's behaviour and
// this card does not reopen it, so the narrow pass asserts what this card is
// answerable for.
func assertNoRuleRunsPastTheEdge(t *testing.T, block sweptBlock, tag, separator string, columns []int) {
	t.Helper()
	rules := sweptRules(separator)
	if len(rules) != len(columns) {
		t.Errorf("%s (%s), locale %s: the separator draws %d rules under %d columns:\n%q",
			block.site, block.label, tag, len(rules), len(columns), separator)
		return
	}
	for i, drawn := range rules {
		if drawn.at != columns[i] {
			t.Errorf("%s (%s), locale %s: the rule under %s begins at display column %d and its heading begins at %d:\n%q",
				block.site, block.label, tag, block.keys[i], drawn.at, columns[i], separator)
			continue
		}
		if drawn.width < 1 {
			t.Errorf("%s (%s), locale %s: the rule under %s draws nothing at all:\n%q", block.site, block.label, tag, block.keys[i], separator)
		}
		if room := sweptWindow - columns[i]; drawn.width > room && room >= 1 {
			t.Errorf("%s (%s), locale %s: the rule under %s draws %d display columns and the window leaves %d before its right edge:\n%q",
				block.site, block.label, tag, block.keys[i], drawn.width, room, separator)
		}
		if i < len(columns)-1 {
			if width := columns[i+1] - columns[i] - sweptGutter; drawn.width > width {
				t.Errorf("%s (%s), locale %s: the rule under %s draws %d display columns and its column is %d wide:\n%q",
					block.site, block.label, tag, block.keys[i], drawn.width, width, separator)
			}
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
// laid out inside the window, with every field inside its own column, draws
// one line per row. What that catches is a last field, which nothing pads,
// running past the edge of a line that had room for every column before it.
//
// Two shapes sit outside it and the check stands aside for both rather than
// excusing them quietly. A block holding a row the window cannot take however
// the columns are chosen draws that row over two lines by design, and so does
// a block holding a field wider than the column it sits in, which is what a
// field the window ruled out of the measurement and a column the narrow-window
// backstop took down both leave behind. The second of those bails out of this
// check entirely, so a column narrower than the rows it holds is a shape this
// assertion does not speak to at all. The narrow-window post-condition in
// TestTheBackstopHoldsWhateverTheWidthsWere is what covers it.
func assertNoRowThatFitsIsContinued(t *testing.T, block sweptBlock, tag string, rowLines []string, columns []int, rows [][]string) {
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
	if len(rowLines) != len(rows) {
		t.Errorf("%s (%s), locale %s: every row of this block fits the window and the block drew %d row lines for %d rows",
			block.site, block.label, tag, len(rowLines), len(rows))
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
	tail := len(columns) - 1
	for i, line := range lines {
		// A block that asks the renderer to break its last column between
		// words draws continuation lines leading at that column's own
		// position. Reading one without knowing that would report an aligned
		// continuation as a first field in the wrong place.
		if block.wrapsTail && next == 0 && len(rows) > 0 && sweptLead(line) == columns[tail] {
			previous := rows[len(rows)-1]
			previous[len(previous)-1] += " " + sweptField(line, columns[tail], -1)
			continue
		}
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

// sweptBlocks are the twenty-two entries of the spec's inventory, at the
// twenty call sites they render through.
func sweptBlocks() []sweptBlock {
	rendered := func(tag, key string) string { return msg.For(tag).T(key) }
	return []sweptBlock{
		{
			site: "render.go:133", label: "the legal moves under a served instruction",
			keys: []string{"column.moves.state", "column.moves.name", "column.moves.direction"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "instructions", w.card)
				return indentedBlock(out, rendered(tag, "instructions.moves"))
			},
		},
		{
			site: "render.go:153", label: "the cards you hold",
			keys: []string{"column.holding.card", "column.holding.title"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "status")
				return indentedBlock(out, rendered(tag, "status.holding"))
			},
		},
		{
			site: "render.go:162", label: "the cards that are blocked",
			keys: []string{"column.blocked.card", "column.blocked.reason"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "status")
				return indentedBlock(out, rendered(tag, "status.blocked"))
			},
		},
		{
			site: "render.go:189", label: "dinah states",
			keys:   []string{"column.states.slug", "column.states.name", "column.states.kind", "column.states.cards", "column.states.owner"},
			varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.healthy, tag, "states"), "")
			},
		},
		{
			site: "render.go:202", label: "dinah ls",
			keys: []string{"column.ls.card", "column.ls.standing", "column.ls.title"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.healthy, tag, "ls"), "")
			},
		},
		{
			site: "render.go:219", label: "dinah query",
			keys: []string{"column.query.card", "column.query.state", "column.query.standing", "column.query.title"},
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.healthy, tag, "query"), "")
			},
		},
		{
			site: "render.go:231", label: "dinah config",
			keys: []string{"column.config.setting", "column.config.value", "column.config.source"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.healthy, tag, "config"), "")
			},
		},
		{
			site: "render.go:259", label: "dinah workbenches",
			keys: []string{"column.workbenches.workbench", "column.workbenches.slug", "column.workbenches.path"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.ambiguous, tag, "workbenches"), "")
			},
		},
		{
			site: "render.go:259", label: "the ambiguous-workbench refusal, written to stderr",
			keys: []string{"column.workbenches.workbench", "column.workbenches.slug", "column.workbenches.path"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRefused(t, w.ambiguous, tag, "status"), "")
			},
		},
		{
			site: "render.go:285", label: "dinah next, a state offering a card",
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
			site: "render.go:285", label: "dinah next, a state offering nothing",
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
			site: "render.go:302", label: "a card's links",
			keys: []string{"column.links.link", "column.links.card"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "show", w.card)
				return indentedBlock(out, rendered(tag, "show.links"))
			},
		},
		{
			site: "render.go:312", label: "a card's comments",
			keys: []string{"column.comments.when", "column.comments.who"}, varies: noCell,
			blanksAreLost: true,
			constantReason: "a timestamp is one format in one time zone, so every comment header draws its stamp " +
				"at the same width; the author in the last column is what this block's assertion rests on",
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "show", w.card)
				return indentedLinesAfter(out, rendered(tag, "show.comments"))
			},
		},
		{
			site: "render.go:337", label: "dinah log",
			keys: []string{"column.log.when", "column.log.action", "column.log.actor", "column.log.detail"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.healthy, tag, "log", w.held), "")
			},
		},
		{
			site: "render.go:353", label: "the slugs check --migrate-slugs assigned",
			keys: []string{"column.slugs.slug", "column.slugs.title"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, sweptStrippedTree(t, w, "slugs-"+tag+"-"+sweptPass), tag, "check", "--migrate-slugs"), "")
			},
		},
		{
			site: "render.go:367", label: "one removed stranded state", varies: noCell,
			constantReason: "this block declares one column and no heading, so it has no column to misplace",
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, sweptStrandedTree(t, w, "stranded-"+tag+"-"+sweptPass), tag, "check", "--migrate-states"), "")
			},
		},
		{
			site: "render.go:503", label: "the states a refusal lists", varies: noCell,
			constantReason: "this block declares one column and no heading, so it has no column to misplace",
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRefused(t, w.healthy, tag, "ls", "nowhere"), "")
			},
		},
		{
			site: "render.go:384", label: "one finding", varies: noCell,
			constantReason: "this block declares one column and no heading, so it has no column to misplace",
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRefused(t, sweptStrippedTree(t, w, "findings-"+tag+"-"+sweptPass), tag, "check"), "")
			},
		},
		{
			site: "render.go:409", label: "catalog coverage",
			keys: []string{"column.catalogs.language", "column.catalogs.translated"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "version", "--catalogs")
				return indentedBlock(out, rendered(tag, "version.catalogs"))
			},
		},
		{
			site: "help.go:108", label: "the command list of bare dinah",
			keys: []string{"column.commands.command", "column.commands.what"}, varies: lastCell,
			noHeadingRow: true,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag)
				return indentedBlock(out, rendered(tag, "help.group.work"))
			},
		},
		{
			site: "help.go:116", label: "the global flag list",
			keys: []string{"column.flags.option", "column.flags.what"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag)
				return indentedBlock(out, rendered(tag, "help.flags"))
			},
		},
		{
			site: "help.go:151", label: "dinah help <command>",
			keys: []string{"column.help.order", "column.help.check", "column.help.refusal"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "help", "add")
				return indentedBlock(out, rendered(tag, "help.refusals"))
			},
		},
		{
			site: "help.go:186", label: "what you may write, on dinah help <command>",
			keys: []string{"column.arguments.argument", "column.arguments.what"}, varies: lastCell,
			wrapsTail: true,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "help", "attach")
				return indentedBlock(out, rendered(tag, "help.arguments"))
			},
		},
		{
			site: "commands.go:391", label: "the guide topics",
			keys: []string{"column.guide.topic", "column.guide.title"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.healthy, tag, "guide"), "")
			},
		},
		{
			site: "render.go:611", label: "the workbench's own fields",
			keys: []string{"column.workbench.field", "column.workbench.value"}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.healthy, tag, "workbench"), "")
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
