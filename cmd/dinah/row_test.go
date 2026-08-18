package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"dinah/internal/textwidth"

	"golang.org/x/term"
	"golang.org/x/text/width"
)

// TestDisplayWidthCountsScreenColumns asserts the per-rune half of the measure:
// a nonspacing mark, an enclosing mark, a format character, and a control
// character occupy nothing, a spacing combining mark occupies one column, an
// East Asian Wide or Fullwidth rune occupies two, and everything else occupies
// one.
//
// The Devanagari cases carry the correction this card was filed with. Half of
// the Devanagari vowel signs are spacing marks and take a column of their own,
// so `हिन्दी` is six runes and five columns, and `कार्ड` is five runes and four.
// A fixture written on the belief that every matra is invisible measures both
// of them short, and the implementer then adjusts the measure until the wrong
// literal passes.
func TestDisplayWidthCountsScreenColumns(t *testing.T) {
	cases := []struct {
		name string
		text string
		want int
	}{
		{"empty", "", 0},
		{"plain ascii", "hello", 5},
		{"cjk", "中文テスト", 10},
		{"fullwidth forms", "ＡＢＣ", 6},
		{"ascii mixed with cjk", "ab中文", 6},
		{"nonspacing mark", "\u0301", 0},
		{"enclosing mark", "\u20DD", 0},
		{"format character", "\u200B", 0},
		{"control character", "\u0001", 0},
		{"spacing combining mark alone", "\u093E", 1},
		{"devanagari hindi", "हिन्दी", 5},
		{"devanagari card", "कार्ड", 4},
		{"base with a combining acute", "中\u0301", 2},
		{"east asian ambiguous counts one", "│", 1},
	}
	for _, c := range cases {
		if got := displayWidth(c.text); got != c.want {
			t.Errorf("%s: displayWidth(%q) = %d, want %d", c.name, c.text, got, c.want)
		}
	}
}

// TestTheUnicodeTablesAgree asserts that both Unicode tables this renderer
// measures with name the release the standard library's own category tables
// came from. A toolchain upgrade moves unicode.Version, and a table left
// behind measures a code point the new release added from its parts.
func TestTheUnicodeTablesAgree(t *testing.T) {
	if textwidth.EmojiUnicodeVersion != unicode.Version {
		t.Errorf("the generated emoji table is at Unicode %s and the standard library is at %s; regenerate it with `go run internal/textwidth/gen_emoji_properties.go` against the matching emoji-data.txt",
			textwidth.EmojiUnicodeVersion, unicode.Version)
	}
	if width.UnicodeVersion != unicode.Version {
		t.Errorf("golang.org/x/text/width is at Unicode %s and the standard library is at %s; move the dependency to the release matching the toolchain",
			width.UnicodeVersion, unicode.Version)
	}
}

// emojiSequenceFiles are the two published files enumerating whole emoji
// sequences, with the number of sequences each holds at the pinned Unicode
// version. The counts are asserted so a parse that stops matching cannot pass
// by sweeping nothing.
var emojiSequenceFiles = []struct {
	// name is the file under testdata.
	name string
	// count is how many sequences it publishes.
	count int
}{
	{name: "emoji-zwj-sequences.txt", count: 1350},
	{name: "emoji-sequences.txt", count: 2314},
}

// TestDisplayWidthMeasuresEveryPublishedEmojiSequence sweeps every sequence
// the three published files hold, with an allowed error of zero.
//
// It is a sweep rather than a list of chosen cases on purpose. A rule that
// used East Asian Width as a proxy for "this is an emoji" measured 703 of the
// zero-width-joiner sequences correctly and the other 911 too wide, and a
// criterion drawn from the cases that already passed reported green over it.
// The variation file is here for the same reason one cycle later: a rule that
// consumed the text presentation selector as though it asked for the emoji
// glyph measured every text presentation sequence a column too wide, and the
// two sequence files were unaffected by that clause and stayed green.
func TestDisplayWidthMeasuresEveryPublishedEmojiSequence(t *testing.T) {
	for _, file := range emojiSequenceFiles {
		sequences := readEmojiSequences(t, file.name)
		if len(sequences) != file.count {
			t.Fatalf("%s: read %d sequences, want %d; the parse or the vendored file has moved", file.name, len(sequences), file.count)
		}
		for _, sequence := range sequences {
			if got := displayWidth(sequence); got != 2 {
				t.Errorf("%s: displayWidth(%q) = %d, want 2 (%s)", file.name, sequence, got, codePoints(sequence))
			}
		}
	}

	emojiStyle, textStyle := readVariationSequences(t)
	const variationCount = 354
	if len(emojiStyle) != variationCount || len(textStyle) != variationCount {
		t.Fatalf("emoji-variation-sequences.txt: read %d emoji style and %d text style, want %d of each", len(emojiStyle), len(textStyle), variationCount)
	}
	for _, sequence := range emojiStyle {
		if got := displayWidth(sequence); got != 2 {
			t.Errorf("emoji presentation sequence %q measured %d, want 2 (%s)", sequence, got, codePoints(sequence))
		}
	}
	narrow, wide := 0, 0
	for _, sequence := range textStyle {
		base := []rune(sequence)[0]
		want := 1
		switch width.LookupRune(base).Kind() {
		case width.EastAsianWide, width.EastAsianFullwidth:
			want = 2
			wide++
		default:
			narrow++
		}
		if got := displayWidth(sequence); got != want {
			t.Errorf("text presentation sequence %q measured %d, want %d (%s)", sequence, got, want, codePoints(sequence))
		}
	}
	if narrow != 213 || wide != 141 {
		t.Errorf("the text presentation split moved: %d narrow bases and %d wide, want 213 and 141", narrow, wide)
	}
}

// TestDisplayWidthFoldsOnlyEmoji asserts the controls that keep the fold off
// text. A joiner between two letters joins nothing, wide text is not folded to
// one glyph because it is wide, and the two variation selectors mean opposite
// things.
func TestDisplayWidthFoldsOnlyEmoji(t *testing.T) {
	cases := []struct {
		name string
		text string
		want int
	}{
		{"joiner between two devanagari letters", "क\u200Dष", 2},
		{"non-joiner between two devanagari letters", "क\u200Cष", 2},
		{"joiner between two ideographs", "中\u200D文", 4},
		{"heart with no selector", "\u2764", 1},
		{"heart asked for as emoji", "\u2764\uFE0F", 2},
		{"heart asked for as text", "\u2764\uFE0E", 1},
		{"copyright asked for as text", "\u00A9\uFE0E", 1},
		{"watch asked for as text", "\u231A\uFE0E", 2},
		{"keycap as emoji", "1\uFE0F\u20E3", 2},
		{"keycap as text", "1\uFE0E\u20E3", 1},
		{"flag", "\U0001F1EF\U0001F1F5", 2},
		{"thumbs up with a skin tone", "\U0001F44D\U0001F3FD", 2},
		{"three person family", "\U0001F468\u200D\U0001F469\u200D\U0001F467", 2},
		{"woman health worker", "\U0001F469\u200D\u2695\uFE0F", 2},
		{"couple with heart", "\U0001F468\u200D\u2764\uFE0F\u200D\U0001F468", 2},
		{"rainbow flag", "\U0001F3F3\uFE0F\u200D\U0001F308", 2},
	}
	for _, c := range cases {
		if got := displayWidth(c.text); got != c.want {
			t.Errorf("%s: displayWidth(%q) = %d, want %d (%s)", c.name, c.text, got, c.want, codePoints(c.text))
		}
	}
}

// codePoints spells a string as its code points, which is what a reader needs
// when a sweep reports a sequence a terminal renders as one glyph.
func codePoints(text string) string {
	var b strings.Builder
	for i, r := range []rune(text) {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString("U+" + strings.ToUpper(strconv.FormatInt(int64(r), 16)))
	}
	return b.String()
}

// readEmojiSequences reads every sequence one published file enumerates. A
// line naming a range of single code points contributes one sequence per code
// point in the range, which is how the published files spell a run of
// single-code-point emoji.
func readEmojiSequences(t *testing.T, name string) []string {
	t.Helper()
	var sequences []string
	forEachDataLine(t, name, func(fields []string) {
		sequences = append(sequences, expandCodePointField(t, fields[0])...)
	})
	return sequences
}

// readVariationSequences reads the variation file into its two styles. The
// file publishes both styles of each base, and reading only one of them is
// what hid a defect for a review cycle.
func readVariationSequences(t *testing.T) (emojiStyle, textStyle []string) {
	t.Helper()
	forEachDataLine(t, "emoji-variation-sequences.txt", func(fields []string) {
		if len(fields) < 2 {
			return
		}
		sequences := expandCodePointField(t, fields[0])
		if len(sequences) != 1 {
			return
		}
		switch strings.TrimSpace(fields[1]) {
		case "emoji style":
			emojiStyle = append(emojiStyle, sequences[0])
		case "text style":
			textStyle = append(textStyle, sequences[0])
		}
	})
	return emojiStyle, textStyle
}

// forEachDataLine hands each data line of a vendored Unicode file to visit,
// with its comment stripped and its semicolon-separated fields split.
func forEachDataLine(t *testing.T, name string, visit func(fields []string)) {
	t.Helper()
	file, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if cut := strings.IndexByte(line, '#'); cut >= 0 {
			line = line[:cut]
		}
		fields := strings.Split(line, ";")
		if len(fields) < 2 || strings.TrimSpace(fields[0]) == "" {
			continue
		}
		visit(fields)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
}

// expandCodePointField reads one code point field: a run of hexadecimal code
// points making one sequence, or a `lo..hi` range making one sequence per code
// point.
func expandCodePointField(t *testing.T, field string) []string {
	t.Helper()
	spelling := strings.TrimSpace(field)
	if lo, hi, ok := strings.Cut(spelling, ".."); ok {
		first := parseCodePoint(t, lo)
		last := parseCodePoint(t, hi)
		expanded := make([]string, 0, last-first+1)
		for r := first; r <= last; r++ {
			expanded = append(expanded, string(r))
		}
		return expanded
	}
	var sequence []rune
	for _, word := range strings.Fields(spelling) {
		sequence = append(sequence, parseCodePoint(t, word))
	}
	return []string{string(sequence)}
}

// parseCodePoint reads one hexadecimal code point.
func parseCodePoint(t *testing.T, spelling string) rune {
	t.Helper()
	value, err := strconv.ParseUint(strings.TrimSpace(spelling), 16, 32)
	if err != nil {
		t.Fatalf("code point %q: %v", spelling, err)
	}
	return rune(value)
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
