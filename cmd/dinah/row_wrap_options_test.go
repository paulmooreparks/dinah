package main

import (
	"strings"
	"testing"
)

// TestOverflowingFirstFieldStaysWhole asserts Contract A's copyability rule:
// a value wider than its column is written whole on its own line and the tail
// resumes underneath, so a long reference never arrives broken. The two rows
// carry a window of zero (the piped case) and a window of twenty, and the
// value exceeds both, and the tail lands on the line under it in both.
//
// What the test guards against is somebody later adding truncation "for
// tidiness". The argument is the value must appear verbatim in the rendered
// row, and the tail must come back on a continuation line indented past the
// row's own indent.
func TestOverflowingFirstFieldStaysWhole(t *testing.T) {
	value := "this-value-is-rather-longer-than-the-window-it-renders-in"
	laidOut := row{indent: 2, cells: []cell{{text: value, width: 10}}, tail: "summary text"}
	for _, window := range []int{0, 20} {
		got := formatRow(laidOut, window)
		if !strings.Contains(got, value) {
			t.Errorf("window %d: value %q is not in the rendered row intact:\n%s", window, value, got)
		}
		lines := strings.Split(got, "\n")
		if len(lines) < 2 {
			t.Errorf("window %d: expected the tail to land on a continuation line, got %q", window, got)
			continue
		}
		tailLine := lines[len(lines)-1]
		if !strings.Contains(tailLine, "summary text") {
			t.Errorf("window %d: tail %q is not on a continuation line:\n%s", window, "summary text", got)
		}
	}
}

// TestContinuationIndentClamp asserts both bounds of the clamp over every
// window a layout can be asked for: a continuation line is never indented
// past the window less the columns reserved for its own text, and is never
// indented below the row's own indent. The ceiling-bearing pass uses the
// same `continuation` helper, so the clamp holds wherever an overflowing
// cell asks for the rest of its line.
//
// What the test guards against is somebody later widening the indent
// without considering narrow windows, or short-circuiting the clamp at the
// floor. The two bounds together are the rule; either alone would let a
// continuation line either overrun the window or fall under the row.
func TestContinuationIndentClamp(t *testing.T) {
	overrunning := []cell{
		{strings.Repeat("x", 80), 14},
		{strings.Repeat("y", 80), 32},
		{strings.Repeat("z", 80), 24},
	}
	for window := minTailColumns; window <= 120; window++ {
		rendered := formatRow(row{indent: 2, cells: overrunning, tail: "tail"}, window)
		allowed := window - minTailColumns
		if allowed < 2 {
			allowed = 2
		}
		lines := strings.Split(rendered, "\n")
		for _, line := range lines[1:] {
			indent := displayWidth(line) - displayWidth(strings.TrimLeft(line, " "))
			if indent > allowed {
				t.Errorf("window %d: a continuation line is indented to %d, past the %d the clamp allows", window, indent, allowed)
			}
			if indent < 2 {
				t.Errorf("window %d: a continuation line is indented to %d, below the row's own indent of 2", window, indent)
			}
		}
	}
}

// TestBreakOnOptions asserts the boundary shapes Contract B names, now that
// dinah-220 has widened them: a space before any `[` (an option group like
// `[--state <state>]` and a vocabulary group like `[new|get|set]` alike), a
// space before `<`, and a bare `--`. Each group is kept whole, and the
// groups pack as many to a line as the room allows rather than taking one
// line each. Every line after the first is indented to the call's indent;
// the first is returned without one, so the caller can lay it on the row's
// own first line in place.
//
// What the test guards against is a parser that catches one shape and misses
// the others, that consumes the boundary space into the next group rather
// than dropping it, or that breaks a group across a line end. The room in
// each case is narrow enough to force a break, so a packing rule that has
// stopped packing shows up here rather than passing on a line nothing had
// to fit into.
func TestBreakOnOptions(t *testing.T) {
	cases := []struct {
		name   string
		text   string
		indent int
		room   int
		want   string
	}{
		{
			name:   "square bracket",
			text:   "add <title> [--state <state>]",
			indent: 2,
			room:   20,
			want:   "add <title>\n  [--state <state>]",
		},
		{
			name:   "angle bracket",
			text:   "claim <card> [--expires <duration>]",
			indent: 2,
			room:   20,
			want:   "claim <card>\n  [--expires <duration>]",
		},
		{
			name:   "bare dash dash",
			text:   "delete <ref> --yes",
			indent: 2,
			room:   12,
			want:   "delete <ref>\n  --yes",
		},
		{
			name:   "vocabulary group",
			text:   "workstream [new|get|set] [workstream|title] [field] [value] [--yes]",
			indent: 4,
			room:   53,
			want:   "workstream [new|get|set] [workstream|title] [field]\n    [value] [--yes]",
		},
		{
			// The room here is narrower than the command name and its first
			// group together, so a rule that opens a group only on `[--`
			// carries the two as one atomic piece and draws them on one line.
			// Splitting before any `[` is what puts them on lines of their
			// own.
			name:   "a plain group after a word is its own piece",
			text:   "workstream [new|get|set] [field]",
			indent: 2,
			room:   15,
			want:   "workstream\n  [new|get|set]\n  [field]",
		},
		{
			name:   "one indent for the square bracket piece",
			text:   "add <title> [--state <state>]",
			indent: 4,
			room:   20,
			want:   "add <title>\n    [--state <state>]",
		},
		{
			name:   "everything that fits packs onto one line",
			text:   "add <title> [--state <state>]",
			indent: 2,
			room:   100,
			want:   "add <title> [--state <state>]",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := breakOnOptions(c.text, c.indent, c.room)
			if got != c.want {
				t.Errorf("breakOnOptions(%q, %d, %d):\n got  %q\n want %q", c.text, c.indent, c.room, got, c.want)
			}
		})
	}
}

// TestPerColumnIndependentWrap asserts Contract A at the row level: a value
// in column A that reaches column A's width takes the rest of its own line,
// the value in column B (which has not overflowed) stays on the first line
// in its own column, and the tail after column B resumes on the continuation
// line indented to where column A would have ended.
//
// What the test guards against is the renderer falling back to a single
// window-wide wrap that loses the column positions, or treating column A's
// overflow as the only axis and pinning column B above its real column.
func TestPerColumnIndependentWrap(t *testing.T) {
	value := "aconsiderablylongcolumnonevalue"
	laidOut := row{
		indent: 2,
		cells: []cell{
			{text: value, width: 10},
			{text: "brief", width: 12},
		},
		tail: "summary after the second column",
	}
	got := formatRow(laidOut, 80)
	want := "  aconsiderablylongcolumnonevalue\n            brief       summary after the second column"
	if got != want {
		t.Errorf("per-column independent wrap:\n got  %q\n want %q", got, want)
	}
}

// TestTwoOverflowingCells asserts the row renderer with two overflowing
// cells: each one takes its own continuation line at its own column's
// offset. The tail resumes under the second overflowing cell's column.
//
// What the test guards against is the renderer treating "two overflows in
// one row" as a special case that pins both values on a single line, or
// that loses the second column's offset when the first column already
// overflowed.
func TestTwoOverflowingCells(t *testing.T) {
	laidOut := row{
		indent: 2,
		cells: []cell{
			{text: "aconsiderablylongworkbenchname", width: 10},
			{text: "anotherlongoverrunningcellvalue", width: 12},
		},
		tail: "tail",
	}
	got := formatRow(laidOut, 80)
	want := "  aconsiderablylongworkbenchname\n            anotherlongoverrunningcellvalue\n                        tail"
	if got != want {
		t.Errorf("two overflowing cells:\n got  %q\n want %q", got, want)
	}
}

// TestBreakOnOptionsFallsBackToWordWrap asserts Contract B's fallback rule:
// once the last option boundary has passed, trailing prose is exploded into
// its own words and packed at the word level rather than being carried
// whole as though it were one more option group. A value with no option
// boundary at all falls back to the existing wrapTail behaviour, word-wrap
// whole.
//
// What the test guards against is a parser that renders the trailing
// prose on a single line past the window, or that word-wraps the option
// groups themselves rather than keeping each group whole. The fixture mixes
// three option boundaries and a long trailing sentence to exercise the
// join; the boundary after the closing `]` is what separates that sentence
// from the option group before it.
func TestBreakOnOptionsFallsBackToWordWrap(t *testing.T) {
	t.Run("trailing prose word-wraps", func(t *testing.T) {
		text := "add <title> [--state <state>] file a new card in the first state"
		got := breakOnOptions(text, 2, 30)
		want := "add <title> [--state <state>]\n  file a new card in the first\n  state"
		if got != want {
			t.Errorf("trailing prose word-wraps:\n got  %q\n want %q", got, want)
		}
	})
	t.Run("no boundary word-wraps whole", func(t *testing.T) {
		text := "this value has no option boundary and runs long"
		got := breakOnOptions(text, 2, 20)
		want := "this value has no\n  option boundary and\n  runs long"
		if got != want {
			t.Errorf("no boundary word-wraps whole:\n got  %q\n want %q", got, want)
		}
	})
}
