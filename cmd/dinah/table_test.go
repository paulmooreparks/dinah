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
func TestAFieldTheWindowCannotTakeDoesNotWidenItsColumn(t *testing.T) {
	drawn := tableSession(80).tableLines(table{
		indent:  2,
		columns: headed("Setting", "Value", "Source"),
		rows: rowsOf(
			[]string{"lang", "en", "default"},
			[]string{"actor", "alka", "environment"},
			[]string{"workbench", strings.Repeat("p", 70), "search"},
		),
	})
	if drawn[0] != "  Setting    Value  Source" {
		t.Errorf("the heading row is %q, and the seventy-column value should not have widened its column", drawn[0])
	}
	if drawn[2] != "  lang       en     default" {
		t.Errorf("the first row is %q", drawn[2])
	}
	if !strings.HasSuffix(drawn[4], strings.Repeat("p", 70)) {
		t.Errorf("the long value should take the rest of its own line, got %q", drawn[4])
	}
}

// TestTheBackstopNarrowsToTheHeadingAndStops asserts the narrow-window
// backstop and its floor: it takes the widest column down until the window
// leaves room for the text after it, and stops where a column reaches its own
// heading rather than eating it.
func TestTheBackstopNarrowsToTheHeadingAndStops(t *testing.T) {
	drawn := tableSession(30).tableLines(table{
		indent:  2,
		columns: headed("Setting", "Value", "Source"),
		rows: rowsOf(
			[]string{"lang", "a value of some length", "default"},
			[]string{"actor", "alka", "environment"},
		),
	})
	fields := strings.Fields(drawn[0])
	if len(fields) != 3 {
		t.Fatalf("the heading row drew %d fields: %q", len(fields), drawn[0])
	}
	floor := 2 + len("Setting") + tableGutter + len("Value") + tableGutter
	if at := startColumnOf(drawn[0], "Source"); at != floor {
		t.Errorf("the last column begins at display column %d and the headings leave it no room before %d:\n%q", at, floor, drawn[0])
	}
	if at := startColumnOf(drawn[0], "Value"); at != 2+len("Setting")+tableGutter {
		t.Errorf("the backstop narrowed a column below its own heading:\n%q", drawn[0])
	}
	if !strings.HasSuffix(drawn[2], "a value of some length") {
		t.Errorf("a value the narrowed column cannot hold should take the rest of its own line, got %q", drawn[2])
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
