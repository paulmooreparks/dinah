package lsp

import (
	"strings"
	"testing"
)

// candidates is what the prose scanner found on one line, as plain text.
func candidates(slug, line string) []string {
	var found []string
	for _, at := range scanLine(slug, 0, line) {
		found = append(found, at.Text)
	}
	return found
}

// TestTheProseGrammarAdmitsAndRefusesByRule asserts dinah-515 criterion 8's
// first half: the strings section 4.3 names as yielding no candidate yield
// none, and the strings it names as yielding one yield exactly one.
//
// foo-8 is the row that bites. Card 8 exists on the workbench this project
// runs on and bench.ResolveCard answers it, because that resolver splits at
// the last dash and keys on the number alone. Requiring the workbench's own
// slug exactly is what refuses it here, and the population that tolerance
// would annotate grows as a workbench does rather than being a fixed set.
func TestTheProseGrammarAdmitsAndRefusesByRule(t *testing.T) {
	const slug = "dinah"
	const uuid = "550e8400-e29b-41d4-a716-446655440000"
	forty := strings.Repeat("ab", 20)
	thirtyTwo := strings.Repeat("0123", 8)

	refused := []struct{ name, write string }{
		{"a bare number", "515"},
		{"a column slug", "spec"},
		{"a column title", "Done"},
		{"a workstream handle with no prefix", "vsix"},
		{"the workbench's own word", "workbench"},
		{"a stale prefix the card resolver would tolerate", "foo-8"},
		{"a twelve-hex run inside a word", "zz0123456789abzz"},
		{"a card reference preceded by a dash", "pre-dinah-515"},
		{"the last group of a UUID", uuid},
		{"a forty-character hexadecimal run", forty},
		{"a thirty-two-character hexadecimal run", thirtyTwo},
		{"a twelve-hex substring of a longer run", "0123456789abcdef0123"},
	}
	swept := 0
	for _, row := range refused {
		if got := candidates(slug, row.write); len(got) != 0 {
			t.Errorf("%s (%q) yielded %v, wanted nothing", row.name, row.write, got)
		}
		swept++
	}
	if swept != len(refused) {
		t.Errorf("swept %d refusing rows, wanted %d", swept, len(refused))
	}

	admitted := []struct{ name, write, want string }{
		{"a card reference", "dinah-515", "dinah-515"},
		{"a bare identifier", "0dff709e0f3c", "0dff709e0f3c"},
		{"a workstream reference", "workstream/vsix", "workstream/vsix"},
		{"a checklist item", "dinah-506/questions/1", "dinah-506/questions/1"},
		{"an attachment's payload", "dinah-515/attachments/1/payload", "dinah-515/attachments/1/payload"},
	}
	swept = 0
	for _, row := range admitted {
		got := candidates(slug, row.write)
		if len(got) != 1 || got[0] != row.want {
			t.Errorf("%s (%q) yielded %v, wanted exactly [%q]", row.name, row.write, got, row.want)
		}
		swept++
	}
	if swept != len(admitted) {
		t.Errorf("swept %d admitting rows, wanted %d", swept, len(admitted))
	}
}

// TestTheBoundaryRuleRunsAheadOfTheTrim asserts dinah-515 criterion 8's
// second half, against the table in section 4.3. A candidate the boundary
// rule refuses is never trimmed, so the four strings ending in a character
// inside the boundary set yield nothing however the trim would have read
// them, and the rest each yield the untrimmed reference.
func TestTheBoundaryRuleRunsAheadOfTheTrim(t *testing.T) {
	const slug = "dinah"
	rows := []struct {
		write string
		want  string
		why   string
	}{
		{"dinah-515.", "dinah-515", "the boundary passes, a full stop being outside the set, and the trim removes nothing from a candidate ending in a digit"},
		{"dinah-515/", "dinah-515", "the tail needs a segment after the slash and matches none, and a slash is outside the boundary set"},
		{"dinah-515's", "dinah-515", "the boundary passes, an apostrophe being outside the set"},
		{"dinah-506/questions/1.", "dinah-506/questions/1", "the tail segment consumes the full stop and the trim strips it"},
		{"dinah-506/questions/1-", "dinah-506/questions/1", "the tail segment consumes the dash, so it never reaches the boundary rule"},
		{"dinah-506/questions/1_", "dinah-506/questions/1", "the same, for an underscore"},
		{"dinah-506/questions/1...", "dinah-506/questions/1", "the trim strips a run rather than one character"},
		{"dinah-506/questions/1/", "dinah-506/questions/1", "the tail matches no further segment"},
		{"workstream/vsix.", "workstream/vsix", "head three, a full stop being outside the set"},
		{"dinah-515-", "", "the boundary refuses, a dash being inside the set, and the trim never runs"},
		{"dinah-515_", "", "the boundary refuses, an underscore being inside the set"},
		{"dinah-515-516", "", "the boundary refuses at the second dash, so a prose range annotates neither end"},
		{"workstream/vsix-", "", "head three carries no tail, so the dash reaches the boundary rule"},
	}
	swept := 0
	for _, row := range rows {
		got := candidates(slug, row.write)
		switch {
		case row.want == "":
			if len(got) != 0 {
				t.Errorf("%q yielded %v, wanted nothing, because %s", row.write, got, row.why)
			}
		case len(got) != 1 || got[0] != row.want:
			t.Errorf("%q yielded %v, wanted exactly [%q], because %s", row.write, got, row.want, row.why)
		}
		swept++
	}
	if swept != len(rows) {
		t.Errorf("swept %d rows, wanted %d", swept, len(rows))
	}
}

// TestFencedBlocksAreExcludedAndIndentedOnesAreNot asserts dinah-515
// criterion 9: a reference inside a fenced code block yields nothing, one
// inside an inline code span yields a candidate, and one inside a four-space
// indented block yields a candidate too, the last pinning the looseness this
// card accepted knowingly rather than leaving it undeclared.
func TestFencedBlocksAreExcludedAndIndentedOnesAreNot(t *testing.T) {
	const slug = "dinah"
	lines := []string{
		"A span carrying `dinah-1` in prose.",
		"```",
		"dinah-2 inside a fenced block",
		"```",
		"    dinah-3 inside a four-space indented block",
		"~~~",
		"dinah-4 inside a tilde-fenced block",
		"~~~",
	}
	var found []string
	for _, at := range scanProse(slug, lines, 0) {
		found = append(found, at.Text)
	}
	want := []string{"dinah-1", "dinah-3"}
	if len(found) != len(want) {
		t.Fatalf("the scan found %v, wanted %v", found, want)
	}
	for i, got := range found {
		if got != want[i] {
			t.Errorf("the scan found %q at position %d, wanted %q", got, i, want[i])
		}
	}
}

// TestTheTrimSetIsDerivedFromTheSegmentClass asserts what dinah-515 decision
// 17 settles: the characters a candidate's tail is trimmed of are exactly the
// segment characters that are not alphanumeric, and the slash an earlier
// draft wrote into the set is not one of them, because a consumed candidate
// can never end in one.
func TestTheTrimSetIsDerivedFromTheSegmentClass(t *testing.T) {
	var trimmed, segment int
	for c := 0; c < 128; c++ {
		if inSegment(byte(c)) {
			segment++
		}
		if !trimmable(byte(c)) {
			continue
		}
		trimmed++
		if !inSegment(byte(c)) {
			t.Errorf("the trim set carries %q, which no tail segment admits", string(rune(c)))
		}
	}
	if segment == 0 {
		t.Fatal("the segment class admits nothing, so this derivation read nothing")
	}
	if trimmed != 3 {
		t.Errorf("the derived trim set holds %d characters, wanted the three the contract names", trimmed)
	}
	for _, c := range []byte{'.', '-', '_'} {
		if !trimmable(c) {
			t.Errorf("the derived trim set does not carry %q", string(rune(c)))
		}
	}
	if trimmable('/') {
		t.Error("the derived trim set carries a slash, which a consumed candidate can never end in")
	}
}
