package main

import (
	"strings"
	"testing"

	"dinah/internal/msg"
)

// tableSession is a session that renders in the base language at a stated
// window, which is all a table needs to lay itself out.
func tableSession(window int) *session {
	return &session{r: msg.For(msg.Base), width: window}
}

// headed builds the columns of a table from the heading words themselves,
// since these cases are about the measure rather than about the catalog.
func headed(headings ...string) []tableColumn {
	columns := make([]tableColumn, 0, len(headings))
	for _, heading := range headings {
		columns = append(columns, tableColumn{heading: heading})
	}
	return columns
}

// rowsOf builds a table's rows from their fields.
func rowsOf(fields ...[]string) []tableRow {
	rows := make([]tableRow, 0, len(fields))
	for _, one := range fields {
		rows = append(rows, tableRow{fields: one})
	}
	return rows
}

// TestATableOfNoRowsDrawsNothing asserts that an empty table prints nothing at
// all, so the sentence its call site prints instead is the whole answer.
func TestATableOfNoRowsDrawsNothing(t *testing.T) {
	drawn := tableSession(80).tableLines(table{indent: 2, columns: headed("Card", "Title")})
	if len(drawn) != 0 {
		t.Errorf("a table of no rows drew %d lines:\n%s", len(drawn), strings.Join(drawn, "\n"))
	}
}

// TestATableOfOneColumnDrawsNoHeading asserts that a block of one column takes
// neither a heading nor a separator, since one column under a sentence that
// already names it is a list.
func TestATableOfOneColumnDrawsNoHeading(t *testing.T) {
	drawn := tableSession(80).tableLines(table{
		indent:  2,
		columns: listColumn(),
		rows:    rowsOf([]string{"the first finding"}, []string{"the second finding"}),
	})
	want := []string{"  the first finding", "  the second finding"}
	if strings.Join(drawn, "\n") != strings.Join(want, "\n") {
		t.Errorf("a list drew:\n%s\nwant:\n%s", strings.Join(drawn, "\n"), strings.Join(want, "\n"))
	}
}

// TestAColumnNoRowFillsIsDropped asserts the rule that keeps a heading from
// standing over air: a column empty in every row goes, heading and all, and
// the columns after it close up.
func TestAColumnNoRowFillsIsDropped(t *testing.T) {
	drawn := tableSession(80).tableLines(table{
		indent:  2,
		columns: headed("Card", "Standing", "Title"),
		rows: rowsOf(
			[]string{"fx-1", "", "the first card"},
			[]string{"fx-2", "", "the second card"},
		),
	})
	if len(drawn) < 2 {
		t.Fatalf("the table drew %d lines", len(drawn))
	}
	if strings.Contains(drawn[0], "Standing") {
		t.Errorf("the heading of a column no row fills was drawn: %q", drawn[0])
	}
	if drawn[0] != "  Card  Title" {
		t.Errorf("the heading row is %q, and dropping the empty column closes the columns after it up", drawn[0])
	}
	if drawn[2] != "  fx-1  the first card" {
		t.Errorf("the first row is %q", drawn[2])
	}
}

// TestTheLastColumnIsMeasuredForItsRuleAlone asserts the operator's ruling and
// the reading the spec takes of it: the rule under a last column is as wide as
// the column, measured from the longest of its heading and its values, and
// that width reaches nothing else. The fields under it stay unpadded, so no
// row picks up the trailing run of spaces this card removes.
func TestTheLastColumnIsMeasuredForItsRuleAlone(t *testing.T) {
	drawn := tableSession(80).tableLines(table{
		indent:  2,
		columns: headed("Card", "Standing", "Title"),
		rows: rowsOf(
			[]string{"fx-1", "active", "a title of twenty-five col"},
			[]string{"fx-2", "ready", "short"},
		),
	})
	rules := strings.Fields(drawn[1])
	if len(rules) != 3 {
		t.Fatalf("the separator drew %d rules:\n%q", len(rules), drawn[1])
	}
	if len(rules[2]) != len("a title of twenty-five col") {
		t.Errorf("the rule under the last column draws %d and its widest value draws %d:\n%q",
			len(rules[2]), len("a title of twenty-five col"), drawn[1])
	}
	if len(rules[1]) != len("Standing") {
		t.Errorf("the rule under Standing draws %d, and a heading is content of its column too:\n%q", len(rules[1]), drawn[1])
	}
	for _, line := range drawn {
		if strings.HasSuffix(line, " ") {
			t.Errorf("a line ends in a space: %q", line)
		}
	}
}

// TestARuleStopsAtTheDisplayEdge asserts the clamp: a rule draws at its
// column's width or at what the window leaves, whichever is smaller, and the
// heading row and the value rows are what they are without it.
func TestARuleStopsAtTheDisplayEdge(t *testing.T) {
	wide := strings.Repeat("x", 60)
	drawn := tableSession(40).tableLines(table{
		indent:  2,
		columns: headed("Card", "Title"),
		rows: rowsOf(
			[]string{"fx-1", wide},
			[]string{"fx-2", "short"},
		),
	})
	rules := strings.Fields(drawn[1])
	if len(rules) != 2 {
		t.Fatalf("the separator drew %d rules:\n%q", len(rules), drawn[1])
	}
	at := displayWidth(drawn[1]) - len(rules[1])
	if at+len(rules[1]) != 40 {
		t.Errorf("the rule under the last column ends at display column %d and the window is 40 wide:\n%q", at+len(rules[1]), drawn[1])
	}
	if !strings.HasSuffix(drawn[2], wide) {
		t.Errorf("the clamp shortened a value as well as its rule:\n%q", drawn[2])
	}
}

// TestAFieldTheWindowCannotTakeDoesNotWidenItsColumn asserts the drop rule and
// the case it exists for: one long value among short ones takes its own line
// rather than pushing every other row's columns out behind it.
//
// It reads the widths the measure chose and the lines they draw, rather than
// the whole block. The table this builds stacks, since Value stands at its own
// heading while a field under it reaches the column, and the drop rule is
// about which widths were chosen rather than about which of the two forms the
// window leaves room for.
func TestAFieldTheWindowCannotTakeDoesNotWidenItsColumn(t *testing.T) {
	long := strings.Repeat("p", 70)
	laid := measure(table{
		indent:  2,
		columns: headed("Setting", "Value", "Source"),
		rows: rowsOf(
			[]string{"lang", "en", "default"},
			[]string{"actor", "alka", "environment"},
			[]string{"workbench", long, "search"},
		),
	}, 80)
	narrowToWindow(&laid)
	if laid.widths[1] != displayWidth("Value") {
		t.Errorf("the Value column measured %d and its heading draws %d, so the seventy-column value widened its column", laid.widths[1], displayWidth("Value"))
	}
	if laid.widths[0] != displayWidth("workbench") {
		t.Errorf("the Setting column measured %d and its widest value draws %d", laid.widths[0], displayWidth("workbench"))
	}
	if line := laid.rowLine(laid.rows[0]); line != "  lang       en     default" {
		t.Errorf("the first row is %q", line)
	}
	if lines := splitLines(laid.rowLine(laid.rows[2])); !strings.HasSuffix(lines[0], long) {
		t.Errorf("the long value should take the rest of its own line, got %q", lines[0])
	}
}

// TestTheBackstopNarrowsToTheHeadingAndStops asserts the narrow-window
// backstop and its floor: it takes the widest column down until the window
// leaves room for the text after it, and stops where a column reaches its own
// heading rather than eating it.
// It reads the widths the backstop left rather than the block they draw,
// since a table whose columns all stand at their headings while a value still
// reaches one of them is the stacked case, and the floor is about the widths.
func TestTheBackstopNarrowsToTheHeadingAndStops(t *testing.T) {
	laid := measure(table{
		indent:  2,
		columns: headed("Setting", "Value", "Source"),
		rows: rowsOf(
			[]string{"lang", "a value of some length", "default"},
			[]string{"actor", "alka", "environment"},
		),
	}, 30)
	narrowToWindow(&laid)
	if laid.widths[0] != displayWidth("Setting") {
		t.Errorf("the Setting column stands at %d and its heading draws %d, so the backstop stopped short of the floor or ate it", laid.widths[0], displayWidth("Setting"))
	}
	if laid.widths[1] != displayWidth("Value") {
		t.Errorf("the Value column stands at %d and its heading draws %d, so the backstop narrowed a column below its own heading", laid.widths[1], displayWidth("Value"))
	}
	if lines := splitLines(laid.rowLine(laid.rows[0])); !strings.HasSuffix(lines[0], "a value of some length") {
		t.Errorf("a value the narrowed column cannot hold should take the rest of its own line, got %q", lines[0])
	}
}

// TestTheGutterSurvivesAFieldOneColumnWiderThanItsOwn asserts the case that
// falls between padding a field and giving it its own line. A field exactly
// one column wider than what its column measured is too wide to leave the
// gutter behind it and too narrow to reach the next column, so the column
// widens by one and the gutter comes back.
func TestTheGutterSurvivesAFieldOneColumnWiderThanItsOwn(t *testing.T) {
	drawn := tableSession(40).tableLines(table{
		indent:  2,
		columns: headed("Workbench", "Slug", "Path"),
		rows: rowsOf(
			[]string{"作業台管理", "wb", strings.Repeat("p", 40)},
			[]string{"other", "ot", strings.Repeat("q", 40)},
		),
	})
	for _, line := range drawn {
		if strings.Contains(line, "作業台管理 wb") {
			t.Errorf("a field one column wider than its own column ate the gutter behind it:\n%q", line)
		}
	}
}

// backstopFixture is one table the narrow-window post-condition is asserted
// over, with a name a failure can report.
type backstopFixture struct {
	name  string
	table table
}

// backstopFixtures are the shapes a narrow window is hardest on: a listing of
// ordinary state names, where many values sit a column or two apart, and a
// listing whose slugs draw a contiguous run of widths, where every width
// between a narrowed column's floor and its measured width is present in some
// row.
func backstopFixtures() []backstopFixture {
	states := table{indent: 2, columns: headed("Slug", "Name", "Kind", "Cards", "Owner")}
	for _, slug := range []string{
		"intake", "spec", "design-review", "build-queue", "implement",
		"code-review", "test", "operator-review", "merge", "done",
	} {
		states.rows = append(states.rows, tableRow{fields: []string{slug, slug, "work", "3", "agent"}})
	}
	run := table{indent: 2, columns: headed("Slug", "Name", "Kind", "Cards", "Owner")}
	for width := 4; width <= 16; width++ {
		slug := strings.Repeat("s", width)
		run.rows = append(run.rows, tableRow{fields: []string{slug, slug, "work", "12", "operator"}})
	}
	fresh := table{
		indent:  2,
		columns: headed("Slug", "Name", "Kind", "Cards", "Owner"),
		rows: rowsOf(
			[]string{"intake", "Intake", "intake", "0", "agent"},
			[]string{"doing", "Doing", "work", "0", "agent"},
			[]string{"done", "Done", "done", "0", "agent"},
		),
	}
	return []backstopFixture{
		{name: "ten ordinary state names", table: states},
		{name: "a contiguous run of slug widths", table: run},
		{name: "a fresh workbench's three states", table: fresh},
	}
}

// leadColumns is the display column the last column starts at: the indent and
// every column before the last one with its own gutter after it. It is what
// both backstop assertions compare against the window, from opposite sides.
func leadColumns(laid laidTable) int {
	lead := laid.indent
	for c := 0; c < len(laid.widths)-1; c++ {
		lead += laid.widths[c] + tableGutter
	}
	return lead
}

// TestTheBackstopHoldsWhateverTheWidthsWere asserts the post-condition the
// narrow-window backstop exists for, read off the laid-out table rather than
// off one rendering: the columns before the last one either leave the room the
// tail asks for, or every one of them stands at its own heading, with nothing
// in between.
//
// This is the assertion the card shipped its first pass without, and the class
// it catches is a pass that widens a column after the backstop has narrowed
// it. Such a pass climbs back one value at a time, so the columns end up where
// the measure put them and the last column starts past the room the window has
// for it. Every other check on this card reads a rendering, and a rendering of
// re-widened columns is indistinguishable from a rendering of a block that was
// always that wide, so the class went unseen. Reading the widths
// against the window is what separates the two.
func TestTheBackstopHoldsWhateverTheWidthsWere(t *testing.T) {
	for _, fixture := range backstopFixtures() {
		for window := minTailColumns; window <= 80; window++ {
			laid := tableSession(window).layOut(fixture.table)
			lead := leadColumns(laid)
			if lead+tailRoom(laid) <= window {
				continue
			}
			for c := 0; c < len(laid.widths)-1; c++ {
				floor := displayWidth(laid.columns[c].heading)
				if laid.widths[c] <= floor {
					continue
				}
				t.Errorf("%s at a window of %d: the columns before the last one run to display column %d and leave %d of the window after them, while column %d stands at %d over a heading of %d",
					fixture.name, window, lead, window-lead, c, laid.widths[c], floor)
			}
		}
	}
}

// TestTheBackstopStandsAsideWhileTheRowsFit asserts the other half of the
// backstop's contract, which the post-condition above cannot see: while the
// window can hold the widths the measure chose, the backstop narrows nothing
// at all.
//
// The post-condition is satisfied by a backstop that fires far too early,
// since narrowing a table that already fitted still leaves every column with
// room after it. That is the shape the card shipped: the backstop reserved a
// flat minTailColumns for the tail whatever the last column had measured, so a
// listing needing 38 display columns was squeezed at a window of 52 and broken
// apart at 49. Every check on this card read either a rendering or the
// post-condition, and both call an early squeeze correct, so the two
// assertions are read together or not at all.
//
// It compares the widths a reader gets against the widths the measure chose at
// the same window, so the fixture's own arithmetic is never restated here and
// a fixture can be added without a number being computed by hand.
func TestTheBackstopStandsAsideWhileTheRowsFit(t *testing.T) {
	for _, fixture := range backstopFixtures() {
		for window := minTailColumns; window <= 120; window++ {
			chosen := measure(fixture.table, window)
			needed := leadColumns(chosen) + chosen.widths[len(chosen.widths)-1]
			if needed > window {
				continue
			}
			laid := tableSession(window).layOut(fixture.table)
			for c := range laid.widths {
				if laid.widths[c] == chosen.widths[c] {
					continue
				}
				t.Errorf("%s at a window of %d: the rows measure %d display columns and fit, and the backstop narrowed column %d from %d to %d",
					fixture.name, window, needed, c, chosen.widths[c], laid.widths[c])
			}
		}
	}
}

// TestATableNoRowFillsAtAllKeepsNoColumns asserts the far end of the
// empty-column rule, which is where layout runs with nothing to lay out: every
// column of the table is empty in every row, so all of them go, and what is
// left has no last column for the backstop to read the tail's room from. The
// case is degenerate and no call site draws it, and it reaches layout all the
// same, so the passes hold rather than reaching past the end of the widths.
func TestATableNoRowFillsAtAllKeepsNoColumns(t *testing.T) {
	laid := tableSession(30).layOut(table{
		indent:  2,
		columns: headed("Card", "Title"),
		rows:    rowsOf([]string{"", ""}, []string{"", ""}),
	})
	if len(laid.columns) != 0 || len(laid.widths) != 0 {
		t.Errorf("a table no row fills anywhere kept %d columns and %d widths", len(laid.columns), len(laid.widths))
	}
}

// TestASectionSurvivesTheStackedForm asserts what a group label does when the
// table it groups gives up its shape. The section keeps its place and replaces
// the blank line the record it opens would otherwise have drawn, so a stack
// separates its records exactly once however they are introduced.
func TestASectionSurvivesTheStackedForm(t *testing.T) {
	drawn := tableSession(20).tableLines(table{
		indent:  2,
		columns: headed("Slug", "Name"),
		rows: []tableRow{
			{fields: []string{"intake", "Intake of some length"}},
			{section: "WORK", fields: []string{"doing", "Doing along name"}},
		},
	})
	want := []string{
		"  Slug  intake",
		"  Name  Intake of some length",
		"",
		"WORK",
		"  Slug  doing",
		"  Name  Doing along name",
	}
	if strings.Join(drawn, "\n") != strings.Join(want, "\n") {
		t.Errorf("a stack carrying a section drew:\n%s\nwant:\n%s", strings.Join(drawn, "\n"), strings.Join(want, "\n"))
	}
}

// TestAStackedRecordDropsAFieldHoldingNoText asserts that a label over nothing
// is not drawn, and that the record's other values stay where every other
// record's do.
func TestAStackedRecordDropsAFieldHoldingNoText(t *testing.T) {
	drawn := tableSession(30).tableLines(table{
		indent:  2,
		columns: headed("Card", "Standing", "Title"),
		rows: rowsOf(
			[]string{"demo-1", "ready", "a card of some length"},
			[]string{"demo-2", "", "a second card of some length"},
		),
	})
	want := []string{
		"  Card      demo-1",
		"  Standing  ready",
		"  Title     a card of some length",
		"",
		"  Card      demo-2",
		"  Title     a second card of some length",
	}
	if strings.Join(drawn, "\n") != strings.Join(want, "\n") {
		t.Errorf("a stack holding an empty field drew:\n%s\nwant:\n%s", strings.Join(drawn, "\n"), strings.Join(want, "\n"))
	}
}

// TestARuleKeepsOneColumnWhereTheWindowLeavesNone asserts the floor under the
// clamp: a column starting at or past the right edge still draws a rule of one
// column rather than a negative repeat count.
func TestARuleKeepsOneColumnWhereTheWindowLeavesNone(t *testing.T) {
	laid := laidTable{indent: 2, window: 20, columns: headed("Card"), widths: []int{5}}
	if got := laid.ruleWidth(0, laid.window+10); got != 1 {
		t.Errorf("a rule starting past the right edge draws %d columns, and one is the floor", got)
	}
}

// TestTheCeilingCapsAColumnAtHalfTheWindow asserts what a table opts into by
// declaring hasCeiling: a column no wider than half the window it draws in,
// with a value too wide for the capped column breaking onto its own line and
// the field after it resuming ceilingContinuationIndent columns past the
// row's own indent rather than under the column it belongs to.
func TestTheCeilingCapsAColumnAtHalfTheWindow(t *testing.T) {
	long := strings.Repeat("x", 60)
	laid := tableSession(100).layOut(table{
		indent: 2, columns: headed("Command", "What"), labels: labelInTheStack,
		hasCeiling: true, ceilingColumn: 0,
		rows: rowsOf(
			[]string{"add <title>", "file a new card"},
			[]string{long, "b"},
		),
	})
	if want := halfWindow(100); laid.widths[0] != want {
		t.Errorf("the capped column measures %d and half the window is %d, so a value the ordinary measure would have widened it past the ceiling did not get clamped", laid.widths[0], want)
	}
	lines := splitLines(laid.rowLine(laid.rows[1]))
	if len(lines) != 2 {
		t.Fatalf("a value wider than the capped column should break onto its own line, got %d lines: %q", len(lines), lines)
	}
	if want := "  " + long; lines[0] != want {
		t.Errorf("the overflowing value's own line is %q, want %q", lines[0], want)
	}
	want := strings.Repeat(" ", 2+ceilingContinuationIndent) + "b"
	if lines[1] != want {
		t.Errorf("the continuation is %q, want it indented %d columns past the row's own indent rather than under the column after it: %q", lines[1], ceilingContinuationIndent, want)
	}
}

// TestTheCeilingNeverNarrowsBelowTheHeading asserts the floor applyCeiling
// keeps: a window narrow enough that half of it falls short of the column's
// own heading leaves the column at its heading rather than under it, the same
// floor narrowToWindow holds elsewhere in this file.
func TestTheCeilingNeverNarrowsBelowTheHeading(t *testing.T) {
	laid := measure(table{
		indent: 2, columns: headed("Command", "What"),
		hasCeiling: true, ceilingColumn: 0,
		rows: rowsOf([]string{"add", "file a new card"}),
	}, 10)
	applyCeiling(&laid)
	if want := displayWidth("Command"); laid.widths[0] != want {
		t.Errorf("the capped column measures %d and its own heading draws %d, so the ceiling narrowed it past the floor", laid.widths[0], want)
	}
}

// TestTheCeilingIgnoresAnOutOfRangeColumn asserts that a table declaring a
// ceilingColumn no column of it carries changes nothing: applyCeiling reads
// widths it does not have and returns rather than indexing past the slice. No
// production table asks for this; it is the defensive half of the same guard
// that also refuses a negative column, exercised directly since nothing
// short of a malformed table ever reaches it.
func TestTheCeilingIgnoresAnOutOfRangeColumn(t *testing.T) {
	for _, column := range []int{-1, 5} {
		laid := measure(table{
			indent: 2, columns: headed("Command", "What"),
			hasCeiling: true, ceilingColumn: column,
			rows: rowsOf([]string{"add <title> [--state <state>]", "file a new card"}),
		}, 100)
		before := laid.widths[0]
		applyCeiling(&laid)
		if laid.widths[0] != before {
			t.Errorf("ceilingColumn %d: applyCeiling changed column 0 from %d to %d, and an out-of-range column should change nothing",
				column, before, laid.widths[0])
		}
	}
}
