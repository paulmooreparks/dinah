package main

import (
	"os"
	"strings"
	"testing"

	"dinah/internal/textwidth"

	"golang.org/x/term"
)

// TestDisplayWidthForwardsToTextwidth asserts that displayWidth is exactly
// what OQ-3 says it is: a one-line call into textwidth.Columns, holding no
// logic of its own. The measure's real tests live in internal/textwidth,
// which is where both of displayWidth's importers (this package and the
// guard in internal/profile) ultimately get their answer from; this test
// only has to prove the forward introduces no drift, not re-derive the
// measure.
func TestDisplayWidthForwardsToTextwidth(t *testing.T) {
	cases := []string{
		"",
		"hello",
		"中文テスト",
		"हिन्दी",
		"\U0001F468\u200D\U0001F469\u200D\U0001F467",
	}
	for _, text := range cases {
		if got, want := displayWidth(text), textwidth.Columns(text); got != want {
			t.Errorf("displayWidth(%q) = %d, want %d (textwidth.Columns)", text, got, want)
		}
	}
}

// TestFormatRowBreaksOnAnOverrunningCell asserts formatRow's cases, which are
// the ones alignedRow carried before this card replaced it: a cell under its
// column pads in place, and a cell that reaches its column gets the line to
// itself with the remaining fields moved to a continuation line indented to
// where the column would have ended, rather than a non-truncating pad pushing
// every later field out of alignment. Two cells overflowing in one row each
// produce their own continuation line at their own column's offset.
//
// Each case states the window it is laid out for, so the continuation clamp is
// exercised as well as the break.
func TestFormatRowBreaksOnAnOverrunningCell(t *testing.T) {
	cases := []struct {
		name   string
		cells  []cell
		window int
		want   string
	}{
		{
			name:  "fits",
			cells: []cell{{"fx", 10}},
			want:  "  fx        rest",
		},
		{
			name:  "placeholder overruns",
			cells: []cell{{"no slug (run check --migrate-slugs)", 10}},
			want:  "  no slug (run check --migrate-slugs)\n            rest",
		},
		{
			name:  "long real slug overruns",
			cells: []cell{{"aconsiderablylongworkbenchnamefortestingcolumnoverrunbehavior", 10}},
			want:  "  aconsiderablylongworkbenchnamefortestingcolumnoverrunbehavior\n            rest",
		},
		{
			name: "two cells overflow in the same row",
			cells: []cell{
				{"aconsiderablylongworkbenchname", 10},
				{"anotherlongoverrunningcellvalue", 8},
			},
			want: "  aconsiderablylongworkbenchname\n            anotherlongoverrunningcellvalue\n                    rest",
		},
		{
			name:   "a wide window clamps nothing",
			cells:  []cell{{"no slug (run check --migrate-slugs)", 10}},
			window: 200,
			want:   "  no slug (run check --migrate-slugs)\n            rest",
		},
		{
			name:   "a narrow window clamps the continuation indent",
			cells:  []cell{{"a workbench title long enough to reach its column", 32}},
			window: 40,
			want:   "  a workbench title long enough to reach its column\n                    rest",
		},
		{
			name:   "the clamp never takes the indent below the row's own",
			cells:  []cell{{"a workbench title long enough to reach its column", 32}},
			window: 20,
			want:   "  a workbench title long enough to reach its column\n  rest",
		},
		{
			name:  "a wide cell is measured on screen rather than in characters",
			cells: []cell{{"中文", 10}},
			want:  "  中文      rest",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := formatRow(row{indent: 2, cells: c.cells, tail: "rest"}, c.window)
			if got != c.want {
				t.Errorf("formatRow at window %d:\n got  %q\n want %q", c.window, got, c.want)
			}
		})
	}
}

// TestFormatRowKeepsEveryContinuationWithinTheWindow asserts both bounds of
// the clamp over every window a layout can be asked for: no continuation line
// is indented past the window less the columns reserved for its own text, and
// none is indented below the row's own indent.
func TestFormatRowKeepsEveryContinuationWithinTheWindow(t *testing.T) {
	overrunning := []cell{
		{strings.Repeat("x", 40), 14},
		{strings.Repeat("y", 40), 32},
		{strings.Repeat("z", 40), 24},
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

// TestFormatRowTruncatesNothing asserts that every run of non-space characters
// the row was given appears intact in what comes back, at every window width
// down to the narrowest one a layout can be asked for. This is the assertion
// that fails if somebody later adds truncation for tidiness.
func TestFormatRowTruncatesNothing(t *testing.T) {
	values := []string{
		`C:\dinah-scratch\a-long-path\.dinah\ac2ee28fb26d\states\doing\state.md`,
		"aconsiderablylongworkbenchnamefortestingcolumnoverrunbehavior",
		"作業台管理という長い題名",
	}
	laidOut := row{
		indent: 2,
		cells:  []cell{{values[0], 14}, {values[1], 32}},
		tail:   values[2],
	}
	for _, window := range []int{20, 40, 200, 0} {
		rendered := formatRow(laidOut, window)
		for _, value := range values {
			if !strings.Contains(rendered, value) {
				t.Errorf("window %d: %q is not in the rendered row intact:\n%s", window, value, rendered)
			}
		}
	}
}

// TestWindowWidthReadsColumns asserts what each shape of COLUMNS states. A
// value that parses and is positive is the width, clamped up to the narrowest
// one a layout can use; anything else states nothing usable, and the width is
// then unknown, which renders unbounded.
//
// The absent case reaches the terminal query, which answers nothing while the
// test binary's own output is captured rather than drawn on a terminal. The
// test says so rather than assuming it, so a run that somehow does hold a
// terminal reports why it disagrees instead of failing.
func TestWindowWidthReadsColumns(t *testing.T) {
	cases := []struct {
		stated string
		want   int
	}{
		{stated: "", want: 0},
		{stated: "   ", want: 0},
		{stated: "abc", want: 0},
		{stated: "0", want: 0},
		{stated: "-5", want: 0},
		{stated: "1", want: 20},
		{stated: "19", want: 20},
		{stated: "20", want: 20},
		{stated: "40", want: 40},
		{stated: " 100 ", want: 100},
	}
	for _, c := range cases {
		t.Setenv("COLUMNS", c.stated)
		if got := windowWidth(); got != c.want {
			t.Errorf("COLUMNS=%q: windowWidth() = %d, want %d", c.stated, got, c.want)
		}
	}
	if term.IsTerminal(int(os.Stdout.Fd())) {
		t.Skip("stdout is a terminal here, so the absent case is answered by the terminal query rather than by the environment")
	}
	t.Setenv("COLUMNS", "40")
	os.Unsetenv("COLUMNS")
	if got := windowWidth(); got != 0 {
		t.Errorf("COLUMNS absent with no terminal to ask: windowWidth() = %d, want 0", got)
	}
}
