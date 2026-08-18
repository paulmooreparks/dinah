package textwidth

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"golang.org/x/text/width"
)

// TestColumnsCountsScreenColumns asserts the per-rune half of the measure: a
// nonspacing mark, an enclosing mark, a format character, and a control
// character occupy nothing, a spacing combining mark occupies one column, an
// East Asian Wide or Fullwidth rune occupies two, and everything else
// occupies one.
//
// The Devanagari cases carry the correction dinah-101 was filed with. Half of
// the Devanagari vowel signs are spacing marks and take a column of their
// own, so `हिन्दी` is six runes and five columns, and `कार्ड` is five runes and
// four. A fixture written on the belief that every matra is invisible
// measures both of them short, and the implementer then adjusts the measure
// until the wrong literal passes.
func TestColumnsCountsScreenColumns(t *testing.T) {
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
		{"nonspacing mark", "́", 0},
		{"enclosing mark", "⃝", 0},
		{"format character", "​", 0},
		{"control character", "", 0},
		{"spacing combining mark alone", "ा", 1},
		{"devanagari hindi", "हिन्दी", 5},
		{"devanagari card", "कार्ड", 4},
		{"base with a combining acute", "中́", 2},
		{"east asian ambiguous counts one", "│", 1},
	}
	for _, c := range cases {
		if got := Columns(c.text); got != c.want {
			t.Errorf("%s: Columns(%q) = %d, want %d", c.name, c.text, got, c.want)
		}
	}
}

// TestTheUnicodeTablesAgree asserts that both Unicode tables this package
// measures with name the release the standard library's own category tables
// came from. A toolchain upgrade moves unicode.Version, and a table left
// behind measures a code point the new release added from its parts.
func TestTheUnicodeTablesAgree(t *testing.T) {
	if EmojiUnicodeVersion != unicode.Version {
		t.Errorf("the generated emoji table is at Unicode %s and the standard library is at %s; regenerate it with `go run internal/textwidth/gen_emoji_properties.go` against the matching emoji-data.txt",
			EmojiUnicodeVersion, unicode.Version)
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

// TestColumnsMeasuresEveryPublishedEmojiSequence sweeps every sequence the
// three published files hold, with an allowed error of zero.
//
// It is a sweep rather than a list of chosen cases on purpose. A rule that
// used East Asian Width as a proxy for "this is an emoji" measured 703 of the
// zero-width-joiner sequences correctly and the other 911 too wide, and a
// criterion drawn from the cases that already passed reported green over it.
// The variation file is here for the same reason one cycle later: a rule that
// consumed the text presentation selector as though it asked for the emoji
// glyph measured every text presentation sequence a column too wide, and the
// two sequence files were unaffected by that clause and stayed green.
func TestColumnsMeasuresEveryPublishedEmojiSequence(t *testing.T) {
	for _, file := range emojiSequenceFiles {
		sequences := readEmojiSequences(t, file.name)
		if len(sequences) != file.count {
			t.Fatalf("%s: read %d sequences, want %d; the parse or the vendored file has moved", file.name, len(sequences), file.count)
		}
		for _, sequence := range sequences {
			if got := Columns(sequence); got != 2 {
				t.Errorf("%s: Columns(%q) = %d, want 2 (%s)", file.name, sequence, got, codePoints(sequence))
			}
		}
	}

	emojiStyle, textStyle := readVariationSequences(t)
	const variationCount = 354
	if len(emojiStyle) != variationCount || len(textStyle) != variationCount {
		t.Fatalf("emoji-variation-sequences.txt: read %d emoji style and %d text style, want %d of each", len(emojiStyle), len(textStyle), variationCount)
	}
	for _, sequence := range emojiStyle {
		if got := Columns(sequence); got != 2 {
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
		if got := Columns(sequence); got != want {
			t.Errorf("text presentation sequence %q measured %d, want %d (%s)", sequence, got, want, codePoints(sequence))
		}
	}
	if narrow != 213 || wide != 141 {
		t.Errorf("the text presentation split moved: %d narrow bases and %d wide, want 213 and 141", narrow, wide)
	}
}

// TestColumnsFoldsOnlyEmoji asserts the controls that keep the fold off text.
// A joiner between two letters joins nothing, wide text is not folded to one
// glyph because it is wide, and the two variation selectors mean opposite
// things.
func TestColumnsFoldsOnlyEmoji(t *testing.T) {
	cases := []struct {
		name string
		text string
		want int
	}{
		{"joiner between two devanagari letters", "क‍ष", 2},
		{"non-joiner between two devanagari letters", "क‌ष", 2},
		{"joiner between two ideographs", "中‍文", 4},
		{"heart with no selector", "❤", 1},
		{"heart asked for as emoji", "❤️", 2},
		{"heart asked for as text", "❤︎", 1},
		{"copyright asked for as text", "©︎", 1},
		{"watch asked for as text", "⌚︎", 2},
		{"keycap as emoji", "1️⃣", 2},
		{"keycap as text", "1︎⃣", 1},
		{"flag", "\U0001F1EF\U0001F1F5", 2},
		{"thumbs up with a skin tone", "\U0001F44D\U0001F3FD", 2},
		{"three person family", "\U0001F468‍\U0001F469‍\U0001F467", 2},
		{"woman health worker", "\U0001F469‍⚕️", 2},
		{"couple with heart", "\U0001F468‍❤️‍\U0001F468", 2},
		{"rainbow flag", "\U0001F3F3️‍\U0001F308", 2},
	}
	for _, c := range cases {
		if got := Columns(c.text); got != c.want {
			t.Errorf("%s: Columns(%q) = %d, want %d (%s)", c.name, c.text, got, c.want, codePoints(c.text))
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
