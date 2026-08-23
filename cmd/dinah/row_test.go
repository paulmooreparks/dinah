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

// TestFormatRowBreaksAWrappedTailBetweenWords asserts the opt-in this card
// adds to the renderer: a row asking for it breaks its tail on a space
// boundary at the window, indents every line after the first under the column
// the tail began at, and a row that does not ask keeps its tail whole.
func TestFormatRowBreaksAWrappedTailBetweenWords(t *testing.T) {
	build := func(wrap bool) row {
		return row{
			indent:   2,
			cells:    []cell{{text: "<ref>", width: 8}},
			tail:     "one two three four five six seven",
			wrapTail: wrap,
		}
	}
	whole := formatRow(build(false), 24)
	if strings.Contains(whole, "\n") {
		t.Errorf("a row that asked for nothing broke its tail:\n%q", whole)
	}
	broken := formatRow(build(true), 24)
	want := "  <ref>   one two three\n          four five six\n          seven"
	if broken != want {
		t.Errorf("the wrapped tail reads\n%q\nand should read\n%q", broken, want)
	}
	for _, line := range strings.Split(broken, "\n") {
		if displayWidth(line) > 24 {
			t.Errorf("a wrapped line is %d columns wide:\n%q", displayWidth(line), line)
		}
	}
}

// TestAWrappedTailBreaksNoWordAndSurvivesAnUnknownWindow asserts the two edges
// of the wrap. A word wider than the room left is written whole and overruns,
// which is the rule the rest of the renderer follows and what keeps a
// reference inside a summary copyable, and a row laid out for an unknown
// window is not wrapped at all, since nothing states where to break.
func TestAWrappedTailBreaksNoWordAndSurvivesAnUnknownWindow(t *testing.T) {
	long := "short " + strings.Repeat("x", 40)
	built := row{indent: 2, cells: []cell{{text: "a", width: 4}}, tail: long, wrapTail: true}
	broken := formatRow(built, 24)
	if !strings.Contains(broken, strings.Repeat("x", 40)) {
		t.Errorf("the long word was broken:\n%q", broken)
	}
	unbounded := formatRow(row{indent: 2, tail: long, wrapTail: true}, 0)
	if strings.Contains(unbounded, "\n") {
		t.Errorf("an unknown window wrapped anyway:\n%q", unbounded)
	}
	noRoom := formatRow(row{indent: 30, tail: "one two three", wrapTail: true}, 24)
	if strings.Contains(noRoom, "\n") {
		t.Errorf("a tail with no room at all was broken:\n%q", noRoom)
	}
}

// TestAWrappedTailIsMeasuredForTheColumnBeforeIt asserts why the wrap needed
// the measure to change with it. A tail that breaks at the window imposes no
// width on its row, so counting it whole would drop the field before it out of
// the measurement and collapse a column a reader can see is wider than its
// heading.
func TestAWrappedTailIsMeasuredForTheColumnBeforeIt(t *testing.T) {
	rows := []tableRow{
		{fields: []string{"[--description <text>]", strings.Repeat("word ", 20)}},
		{fields: []string{"<ref>", "short"}},
	}
	columns := []tableColumn{{heading: "As you write it"}, {heading: "What it is"}}
	wrapped := measure(table{indent: 2, columns: columns, rows: rows, wrapTail: true}, 80)
	if wrapped.widths[0] != displayWidth("[--description <text>]") {
		t.Errorf("the wrapping table measured its first column at %d, want %d", wrapped.widths[0], displayWidth("[--description <text>]"))
	}
	plain := measure(table{indent: 2, columns: columns, rows: rows}, 80)
	if plain.widths[0] != displayWidth("As you write it") {
		t.Errorf("a table that does not wrap measured its first column at %d, want its heading's %d", plain.widths[0], displayWidth("As you write it"))
	}
}

// TestBreakWordsJoinsWordsWithExactlyOneSpace pins two properties of
// breakWords that the guide wrap in guide_wrap.go depends on and that no
// other test in this package asserts.
//
// The single-space join is the load-bearing one, and it matters for more than
// appearance. checkColumnsLineUp runs over both streams of every runCLI
// invocation and folds any line carrying a run of two or more spaces into a
// columnar block, then requires that block to line up. Wrapped guide prose
// passes that check only because breakWords never emits a double space, so a
// breaker that preserved runs of whitespace would start tripping the
// alignment check on every test that renders a guide, far from the change
// that caused it.
//
// The room floor is the second: a room below one returns the text whole
// rather than breaking it a character at a time. That case is unreachable
// through the renderer, since windowWidth clamps to minTailColumns, but
// wrapGuideText is called directly at narrow widths by the guide wrap's own
// tests and rests on it.
//
// Both are pinned here rather than left to whichever card lands second
// because dinah-220 rewrites this function into a delegate over a new
// packTokens helper, and a property nobody wrote down is a property that
// rewrite is free to lose.
func TestBreakWordsJoinsWordsWithExactlyOneSpace(t *testing.T) {
	text := "one  two\tthree\n\nfour"
	if got, want := breakWords(text, 0, 40), "one two three four"; got != want {
		t.Errorf("breakWords rendered %q, want %q", got, want)
	}
	long := strings.Repeat("alpha bravo  charlie\t", 6)
	for _, line := range strings.Split(breakWords(long, 3, 20), "\n") {
		if strings.Contains(strings.TrimLeft(line, " "), "  ") {
			t.Errorf("a wrapped line carries a run of two spaces: %q", line)
		}
	}
	for _, room := range []int{0, -1} {
		if got := breakWords(text, 0, room); got != text {
			t.Errorf("breakWords at room %d returned %q, want the text whole", room, got)
		}
	}
}
