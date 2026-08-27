package rename

import (
	"strings"
	"testing"
)

// diffOf builds a unified diff for one file out of the lines given, so that a
// test states the changed text rather than the format around it. A line
// beginning with -, + or a space is passed through; anything else is context.
func diffOf(file string, oldStart, newStart int, lines ...string) string {
	var b strings.Builder
	b.WriteString("diff --git a/" + file + " b/" + file + "\n")
	b.WriteString("--- a/" + file + "\n")
	b.WriteString("+++ b/" + file + "\n")
	b.WriteString("@@ -" + itoa(oldStart) + ",1 +" + itoa(newStart) + ",1 @@\n")
	for _, line := range lines {
		b.WriteString(line + "\n")
	}
	return b.String()
}

// itoa spells a small non-negative number, which keeps the fixture builder
// free of a strconv import that nothing else in the file needs.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

// TestTokenizeSplitsAnIdentifierIntoItsWords asserts the split that makes an
// identifier legible to the sweep. Without it the word standing in front of
// the replacement inside CrashStates is whatever opened the line, which is a
// comment marker or a keyword, and every renamed identifier in a tree lands in
// one bucket nobody reads.
func TestTokenizeSplitsAnIdentifierIntoItsWords(t *testing.T) {
	cases := []struct {
		name string
		line string
		want []string
	}{
		{
			name: "camel case",
			line: "TestTheTwoUnresolvableCrashColumns",
			want: []string{"Test", "The", "Two", "Unresolvable", "Crash", "Columns"},
		},
		{
			name: "lower camel case",
			line: "liveColumn",
			want: []string{"live", "Column"},
		},
		{
			name: "an all-caps prefix keeps its last letter",
			line: "HTTPColumn",
			want: []string{"HTTP", "Column"},
		},
		{
			name: "an underscore stands alone",
			line: "crash_columns",
			want: []string{"crash", "_", "columns"},
		},
		{
			name: "digits are their own token",
			line: "column12",
			want: []string{"column", "12"},
		},
		{
			name: "punctuation is one token each",
			line: "`column`.",
			want: []string{"`", "column", "`", "."},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []string
			for _, tok := range tokenize(diffLine{number: 1, text: c.line}) {
				got = append(got, tok.text)
			}
			if strings.Join(got, "|") != strings.Join(c.want, "|") {
				t.Errorf("tokenize(%q) = %v, wanted %v", c.line, got, c.want)
			}
		})
	}
}

// TestTokenizeKeepsACombiningMarkInsideItsWord asserts that a word written in
// a script whose vowels are marks on a consonant arrives as one token. Dinah's
// Hindi catalog is that script, and until this held, स्तंभ came apart into its
// consonants with the virama and the anusvara emitted as punctuation between
// them, so no token could ever equal the word a caller was looking for and
// every Hindi rename swept to zero.
func TestTokenizeKeepsACombiningMarkInsideItsWord(t *testing.T) {
	cases := []struct {
		name string
		line string
		want []string
	}{
		{
			name: "a virama and an anusvara stay inside the word",
			line: "स्तंभ",
			want: []string{"स्तंभ"},
		},
		{
			name: "a vowel sign stays inside the word",
			line: "कॉलम",
			want: []string{"कॉलम"},
		},
		{
			name: "a postposition is a word of its own",
			line: "कॉलम में",
			want: []string{"कॉलम", "में"},
		},
		{
			name: "a combining acute stays inside a Latin word",
			line: "café",
			want: []string{"café"},
		},
		{
			name: "punctuation around the word still stands alone",
			line: "\"कॉलम\",",
			want: []string{"\"", "कॉलम", "\"", ","},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []string
			for _, tok := range tokenize(diffLine{number: 1, text: c.line}) {
				got = append(got, tok.text)
			}
			if strings.Join(got, "|") != strings.Join(c.want, "|") {
				t.Errorf("tokenize(%q) = %q, wanted %q", c.line, got, c.want)
			}
		})
	}
}

// TestSweepReadsARenameWrittenInDevanagari asserts the whole of the blocker,
// end to end rather than at the tokenizer alone. This is the rename this very
// card made, and the reviewer who ran the document's own rule against it got
// zero replacements in zero groups and could not tell that from a clean tree.
func TestSweepReadsARenameWrittenInDevanagari(t *testing.T) {
	diff := diffOf("internal/msg/locales/hi.json", 88, 88,
		"-      \"text\": \"कोई कार्ड संग्रहीत स्तंभ में नहीं है\",",
		"+      \"text\": \"कोई कार्ड संग्रहीत कॉलम में नहीं है\",",
	)
	result, err := Sweep(diff, "स्तंभ", "कॉलम")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(result.Replacements) != 1 {
		t.Fatalf("wanted one replacement, got %d: %+v", len(result.Replacements), result.Replacements)
	}
	got := result.Replacements[0]
	if got.Old != "स्तंभ" || got.New != "कॉलम" {
		t.Errorf("wanted स्तंभ replaced by कॉलम, got %q by %q", got.Old, got.New)
	}
	if got.Preceding != "संग्रहीत" {
		t.Errorf("wanted the preceding word संग्रहीत, got %q", got.Preceding)
	}
	if got.Following != "में" {
		t.Errorf("wanted the following word में, got %q", got.Following)
	}
}

// TestSweepRefusesATermItCannotRead asserts the half of the blocker's fix that
// outlives the tokenizer. Folding combining marks into a word makes Devanagari
// work, and it says nothing about the next script or the next caller mistake.
// A term this sweep splits can never match, so the run is guaranteed to report
// zero, and a zero that means "I cannot read this" must not be spelled the
// same way as a zero that means "there is nothing here". The refusal is what
// keeps those two answers apart.
func TestSweepRefusesATermItCannotRead(t *testing.T) {
	diff := diffOf("a.md", 1, 1,
		"-the pending state carries",
		"+the pending column carries",
	)
	cases := []struct {
		name    string
		retired string
		adopted string
		wants   []string
	}{
		{
			name:    "a term of two words",
			retired: "board state",
			adopted: "column",
			wants:   []string{"--old", "board + state", "clean tree"},
		},
		{
			name:    "a term that splits at its case boundary",
			retired: "state",
			adopted: "cardColumn",
			wants:   []string{"--new", "card + Column"},
		},
		{
			name:    "a term carrying punctuation",
			retired: "work-state",
			adopted: "column",
			wants:   []string{"--old", "work + - + state"},
		},
		{
			name:    "an empty term",
			retired: "",
			adopted: "column",
			wants:   []string{"--old", "carries no word"},
		},
		{
			name:    "a stray combining mark on its own",
			retired: "ं",
			adopted: "column",
			wants:   []string{"--old", "not one word"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result, err := Sweep(diff, c.retired, c.adopted)
			if err == nil {
				t.Fatalf("wanted a refusal, got a report of %d replacements", len(result.Replacements))
			}
			for _, want := range c.wants {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("wanted the refusal to carry %q, got %q", want, err.Error())
				}
			}
		})
	}
}

// TestSweepAcceptsATermItCanRead asserts the other side of that refusal, so
// that a guard which refuses everything cannot pass for a guard that refuses
// the right things.
func TestSweepAcceptsATermItCanRead(t *testing.T) {
	diff := diffOf("a.md", 1, 1,
		"-the pending state carries",
		"+the pending column carries",
	)
	for _, pair := range [][2]string{
		{"state", "column"},
		{"State", "Column"},
		{"स्तंभ", "कॉलम"},
		{"Zustand", "Spalte"},
	} {
		if _, err := Sweep(diff, pair[0], pair[1]); err != nil {
			t.Errorf("Sweep(%q, %q): %v", pair[0], pair[1], err)
		}
	}
}

// TestATokenCarriesItsLineAndOffset asserts that a token knows where it came
// from, since a report a reader cannot open is a report they will not use.
func TestATokenCarriesItsLineAndOffset(t *testing.T) {
	tokens := tokenize(diffLine{number: 42, text: "the pending column carries"})
	if len(tokens) != 4 {
		t.Fatalf("wanted four tokens, got %d: %v", len(tokens), tokens)
	}
	third := tokens[2]
	if third.text != "column" {
		t.Fatalf("wanted the third token to be column, got %q", third.text)
	}
	if third.line != 42 {
		t.Errorf("wanted line 42, got %d", third.line)
	}
	if third.column != len("the pending ") {
		t.Errorf("wanted offset %d, got %d", len("the pending "), third.column)
	}
}

// TestSweepReportsTheWordInFrontOfEachReplacement asserts the finding the
// sweep exists for: the replacement is reported with the company it keeps, and
// that company is read off the post-rename side.
func TestSweepReportsTheWordInFrontOfEachReplacement(t *testing.T) {
	diff := diffOf("docs/design/format.md", 700, 702,
		"-leaving the pending state carries at least one `citations` entry. Each",
		"+leaving the pending column carries at least one `citations` entry. Each",
	)
	result, err := Sweep(diff, "state", "column")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(result.Replacements) != 1 {
		t.Fatalf("wanted one replacement, got %d: %+v", len(result.Replacements), result.Replacements)
	}
	got := result.Replacements[0]
	if got.Preceding != "pending" {
		t.Errorf("wanted the preceding word to be pending, got %q", got.Preceding)
	}
	if got.File != "docs/design/format.md" {
		t.Errorf("wanted the file path, got %q", got.File)
	}
	if got.Line != 702 {
		t.Errorf("wanted line 702, got %d", got.Line)
	}
	if got.Old != "state" || got.New != "column" {
		t.Errorf("wanted state becoming column, got %q becoming %q", got.Old, got.New)
	}
	if got.Glued {
		t.Error("wanted a word of its own, got one read as part of an identifier")
	}
}

// TestSweepReadsThePrecedingWordOffAnUnchangedLine asserts the case that
// hard-wrapped prose produces and that a diff taken without context cannot
// answer. The rename fitted inside one line, so the word in front of it sits
// on the line above and reaches the sweep only as context. This is the fourth
// of the four sites dinah-311 repaired, and the one a sweep reading changed
// lines alone reports as having no company at all.
func TestSweepReadsThePrecedingWordOffAnUnchangedLine(t *testing.T) {
	diff := diffOf("docs/design/format.md", 782, 782,
		" whose citation uses a scheme demanding the observation leaves the pending",
		"-state only where that citation recorded the check failing against the",
		"+column only where that citation recorded the check failing against the",
	)
	result, err := Sweep(diff, "state", "column")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(result.Replacements) != 1 {
		t.Fatalf("wanted one replacement, got %d: %+v", len(result.Replacements), result.Replacements)
	}
	if got := result.Replacements[0].Preceding; got != "pending" {
		t.Errorf("wanted the preceding word to be pending, taken from the line above, got %q", got)
	}
	if got := result.Replacements[0].Line; got != 783 {
		t.Errorf("wanted line 783, one past the context line, got %d", got)
	}
}

// TestSweepReadsAReplacementInsideAnIdentifier asserts the shape of the fourth
// defect dinah-287 shipped, a test name calling two conditions columns, and
// asserts that it is bucketed apart from a word of its own. The preceding word
// is the identifier part in front of it rather than the comment marker that
// opens the line.
func TestSweepReadsAReplacementInsideAnIdentifier(t *testing.T) {
	diff := diffOf("internal/verb/beyond_test.go", 1634, 1634,
		"-// TestTheTwoUnresolvableCrashStatesAreReportedAndNotRepaired asserts the row",
		"+// TestTheTwoUnresolvableCrashColumnsAreReportedAndNotRepaired asserts the row",
	)
	result, err := Sweep(diff, "state", "column")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(result.Replacements) != 1 {
		t.Fatalf("wanted one replacement, got %d: %+v", len(result.Replacements), result.Replacements)
	}
	got := result.Replacements[0]
	if got.Preceding != "crash" {
		t.Errorf("wanted the preceding word to be crash, got %q", got.Preceding)
	}
	if !got.Glued {
		t.Error("wanted the replacement read as part of an identifier")
	}
	if got.Old != "States" || got.New != "Columns" {
		t.Errorf("wanted States becoming Columns, got %q becoming %q", got.Old, got.New)
	}
}

// TestAnUnderscoreIsNotGlue asserts that a snake-case name is judged as the
// words it is written as. Its parts are separated in the source, so the reader
// weighs "crash column" rather than an identifier.
func TestAnUnderscoreIsNotGlue(t *testing.T) {
	diff := diffOf("internal/verb/beyond_test.go", 10, 10,
		"-\tcrash_state := 1",
		"+\tcrash_column := 1",
	)
	result, err := Sweep(diff, "state", "column")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(result.Replacements) != 1 {
		t.Fatalf("wanted one replacement, got %d: %+v", len(result.Replacements), result.Replacements)
	}
	if result.Replacements[0].Glued {
		t.Error("wanted an underscore read as a separator rather than as glue")
	}
	if got := result.Replacements[0].Preceding; got != "crash" {
		t.Errorf("wanted the preceding word to be crash, got %q", got)
	}
}

// TestSweepMatchesBothNumbers asserts that one call covers the singular and
// the plural, since a rename produces both and running the sweep twice for one
// rename is how the plural gets forgotten.
func TestSweepMatchesBothNumbers(t *testing.T) {
	diff := diffOf("internal/verb/read.go", 5, 5,
		"-// every state the workbench declares, and the states it hides",
		"+// every column the workbench declares, and the columns it hides",
	)
	result, err := Sweep(diff, "state", "column")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(result.Replacements) != 2 {
		t.Fatalf("wanted two replacements, got %d: %+v", len(result.Replacements), result.Replacements)
	}
	if result.Replacements[1].Old != "states" || result.Replacements[1].New != "columns" {
		t.Errorf("wanted the plural paired, got %q becoming %q",
			result.Replacements[1].Old, result.Replacements[1].New)
	}
}

// TestSweepAlignsARewrittenSentence asserts the general alignment. A run whose
// two sides carry other edits as well cannot be read index for index, and the
// replacement inside it still has to be found, because a rename that rewraps a
// paragraph produces exactly this shape.
func TestSweepAlignsARewrittenSentence(t *testing.T) {
	diff := diffOf("docs/design/format.md", 20, 20,
		"-the pending state carries one entry, and nothing else",
		"+each and every pending column carries one entry",
	)
	result, err := Sweep(diff, "state", "column")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(result.Replacements) != 1 {
		t.Fatalf("wanted one replacement, got %d: %+v", len(result.Replacements), result.Replacements)
	}
	if got := result.Replacements[0].Preceding; got != "pending" {
		t.Errorf("wanted the preceding word to be pending, got %q", got)
	}
}

// TestSweepDoesNotInventAReplacement asserts that a run where the retired word
// was deleted outright, and the new word was never put in its place, reports
// nothing. A sweep that pairs any deletion with any insertion turns every
// rewrite in the range into a finding.
func TestSweepDoesNotInventAReplacement(t *testing.T) {
	diff := diffOf("internal/verb/read.go", 5, 5,
		"-// the state a caller would act on",
		"+// what a caller would act on",
	)
	result, err := Sweep(diff, "state", "column")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(result.Replacements) != 0 {
		t.Errorf("wanted no replacement, got %+v", result.Replacements)
	}
}

// TestSweepReportsARunItCannotAlign asserts that a run too large to align is
// named rather than dropped. A sweep that silently skips the hardest run in a
// diff answers clean for the place a defect is most likely to be, which is the
// failure the whole card is about.
func TestSweepReportsARunItCannotAlign(t *testing.T) {
	wide := strings.Repeat("alpha beta gamma delta ", 400)
	diff := diffOf("internal/msg/locales/hi.json", 1, 1,
		"-"+wide+"state and a rewritten tail",
		"+"+wide+"column and a different tail entirely",
	)
	result, err := Sweep(diff, "state", "column")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(result.Unaligned) != 1 {
		t.Fatalf("wanted one unaligned run, got %d: %+v", len(result.Unaligned), result.Unaligned)
	}
	if !strings.Contains(result.Unaligned[0].Reason, "alignment cap") {
		t.Errorf("wanted the reason to name the cap, got %q", result.Unaligned[0].Reason)
	}
	if result.Unaligned[0].File != "internal/msg/locales/hi.json" {
		t.Errorf("wanted the file named, got %q", result.Unaligned[0].File)
	}
}

// TestAWholesaleRenameTakesTheFastPath asserts that a run whose two sides
// differ only where the rename touched them is aligned index for index. That
// path is what keeps a catalog rewritten on every line clear of the alignment
// cap, so the assertion is that a run far past the cap still reports its
// replacements.
func TestAWholesaleRenameTakesTheFastPath(t *testing.T) {
	var removed, added []string
	for i := 0; i < 400; i++ {
		removed = append(removed, "-the state and the state and the state again")
		added = append(added, "+the column and the column and the column again")
	}
	diff := diffOf("internal/msg/locales/hi.json", 1, 1, append(removed, added...)...)
	result, err := Sweep(diff, "state", "column")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(result.Unaligned) != 0 {
		t.Fatalf("wanted no unaligned run, got %+v", result.Unaligned)
	}
	if len(result.Replacements) != 1200 {
		t.Fatalf("wanted 1200 replacements, got %d", len(result.Replacements))
	}
	if got := result.Replacements[0].Preceding; got != "the" {
		t.Errorf("wanted the preceding word to be the, got %q", got)
	}
}

// TestBucketsPutTheRarestFirst asserts the order the report is read in. A
// legitimate replacement sits in the same phrase hundreds of times and a wrong
// sense sits in a phrase that appears once, so the rare end of the
// distribution is where a reader's attention belongs.
func TestBucketsPutTheRarestFirst(t *testing.T) {
	reps := []Replacement{
		{Preceding: "the", Following: "of", New: "column"},
		{Preceding: "the", Following: "of", New: "column"},
		{Preceding: "the", Following: "of", New: "column"},
		{Preceding: "pending", Following: "carries", New: "column"},
		{Preceding: "a", Following: "named", New: "column"},
		{Preceding: "a", Following: "named", New: "column"},
	}
	buckets := Buckets(reps)
	var order []string
	for _, bucket := range buckets {
		order = append(order, bucket.Label())
	}
	want := "pending column carries|a column named|the column of"
	if strings.Join(order, "|") != want {
		t.Errorf("wanted %q, got %q", want, strings.Join(order, "|"))
	}
}

// TestTheFollowingWordSplitsAnOrdinaryEnglishSentenceOutOfADeterminerGroup
// asserts the correction that made the sweep able to see the plainest kind of
// over-eager rename there is. An ordinary English sentence where the renamed
// word appears as a common noun sits behind the same determiner as the board
// phrase does, so a group keyed on the determiner alone swallows it whole: the
// seven planted in this repository's corpus disappeared into groups of
// hundreds and none of them reached the report. Keyed on the phrase, the same
// sites stand alone at the top.
func TestTheFollowingWordSplitsAnOrdinaryEnglishSentenceOutOfADeterminerGroup(t *testing.T) {
	reps := []Replacement{
		{Preceding: "the", Following: "of", New: "column", File: "a.md", Line: 1},
		{Preceding: "the", Following: "of", New: "column", File: "b.md", Line: 2},
		{Preceding: "the", Following: "of", New: "column", File: "c.md", Line: 3},
		{Preceding: "the", Following: "of", New: "column", File: "d.md", Line: 4},
		// The defect: "the state it left behind" became "the column
		// it left behind". It shares its determiner with the four
		// above and shares no verdict with them.
		{Preceding: "the", Following: "it", New: "column", File: "e.md", Line: 5},
	}
	buckets := Buckets(reps)
	if len(buckets) != 2 {
		t.Fatalf("wanted the determiner group split in two, got %d: %+v", len(buckets), buckets)
	}
	if buckets[0].Label() != "the column it" || len(buckets[0].Sites) != 1 {
		t.Fatalf("wanted the lone defect first, got %q with %d sites", buckets[0].Label(), len(buckets[0].Sites))
	}
	if buckets[0].Sites[0].File != "e.md" {
		t.Errorf("wanted the defect's own site, got %s", buckets[0].Sites[0].File)
	}
}

// TestSweepReadsTheFollowingWordOffAnUnchangedLine asserts that the second
// half of the key reaches across a line boundary the way the first half does.
// Prose here is hard-wrapped, so a replacement that closes a run has the word
// after it on the next line, and that line is unchanged whenever the rename
// fitted inside one line. Without this the phrase would end at the replacement
// and every wrapped site would key on nothing.
func TestSweepReadsTheFollowingWordOffAnUnchangedLine(t *testing.T) {
	diff := diffOf("docs/design/format.md", 782, 782,
		"-carries at least one entry. Leaving the pending state",
		"+carries at least one entry. Leaving the pending column",
		" requires a citation that resolves.",
	)
	result, err := Sweep(diff, "state", "column")
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(result.Replacements) != 1 {
		t.Fatalf("wanted one replacement, got %d: %+v", len(result.Replacements), result.Replacements)
	}
	if got := result.Replacements[0].Following; got != "requires" {
		t.Errorf("wanted the following word to be requires, taken from the line below, got %q", got)
	}
}

// TestABucketSeparatesAnIdentifierFromAWord asserts the split that keeps a
// bucket answerable. "live column" in a comment and LiveColumns in a test name
// share a preceding word and do not share a verdict, and a bucket carrying
// both shows the reader whichever happens to sort first.
func TestABucketSeparatesAnIdentifierFromAWord(t *testing.T) {
	reps := []Replacement{
		{Preceding: "live", Following: "in", New: "Columns", Glued: true},
		{Preceding: "live", Following: "in", New: "Columns", Glued: true},
		{Preceding: "live", Following: "in", New: "column"},
	}
	buckets := Buckets(reps)
	if len(buckets) != 2 {
		t.Fatalf("wanted two buckets, got %d: %+v", len(buckets), buckets)
	}
	if buckets[0].Label() != "live column in" || len(buckets[0].Sites) != 1 {
		t.Errorf("wanted the lone word first, got %q with %d sites", buckets[0].Label(), len(buckets[0].Sites))
	}
	if buckets[1].Label() != "liveColumns in (identifier)" {
		t.Errorf("wanted the identifier bucket marked, got %q", buckets[1].Label())
	}
}

// TestReportNamesItsOwnSize asserts that the report says how much reading it
// is asking for before it asks. A pass whose cost is unknown until it is
// finished is a pass an implementer skips.
func TestReportNamesItsOwnSize(t *testing.T) {
	result := Result{
		Replacements: []Replacement{
			{File: "a.go", Line: 3, Preceding: "pending", Following: "carries", New: "column", Excerpt: "the pending column"},
			{File: "b.go", Line: 4, Preceding: "the", Following: "of", New: "column", Excerpt: "the column"},
			{File: "c.go", Line: 5, Preceding: "the", Following: "of", New: "column", Excerpt: "the column again"},
		},
		Unaligned: []Unaligned{{File: "d.json", Line: 9, Reason: "past the alignment cap"}},
	}
	var out strings.Builder
	if err := Report(&out, result, "state", "column", false); err != nil {
		t.Fatalf("Report: %v", err)
	}
	got := out.String()
	if !strings.HasPrefix(got, "3 replacements of \"state\" by \"column\", in 2 groups") {
		t.Errorf("wanted the header to carry both counts, got %q", firstLine(got))
	}
	// The phrase opens the line and no field is padded to a width. Both
	// halves matter: the phrase is what a reader scans down the left edge,
	// and a padded field counts characters where a terminal counts columns,
	// which drifts on the Devanagari excerpts this sweep's own catalogs
	// produce.
	if !strings.Contains(got, "\npending column carries   1 site   a.go:3  the pending column\n") {
		t.Errorf("wanted the rarest group's line phrase-first and unpadded, got:\n%s", got)
	}
	if !strings.Contains(got, "\nthe column of   2 sites   b.go:4  the column\n") {
		t.Errorf("wanted the common group counted in the plural, got:\n%s", got)
	}
	if !strings.Contains(got, "unaligned run   d.json:9   past the alignment cap") {
		t.Errorf("wanted the unaligned run reported, got:\n%s", got)
	}
	if strings.Contains(got, "c.go:5") {
		t.Errorf("wanted one example per bucket without --all, got:\n%s", got)
	}
}

// TestReportCountsOneOfSomethingInTheSingular asserts that the header does not
// say "1 replacements". A rename in a language that inflects is swept once per
// form, and those extra runs turn up single replacements routinely, so the
// singular case is the ordinary one rather than a curiosity.
func TestReportCountsOneOfSomethingInTheSingular(t *testing.T) {
	result := Result{
		Replacements: []Replacement{
			{File: "hi.json", Line: 1609, Preceding: "अधिक", Following: "में", New: "कॉलमों", Excerpt: "एक से अधिक कॉलमों में"},
		},
	}
	var out strings.Builder
	if err := Report(&out, result, "स्तंभों", "कॉलमों", false); err != nil {
		t.Fatalf("Report: %v", err)
	}
	want := "1 replacement of \"स्तंभों\" by \"कॉलमों\", in 1 group by surrounding phrase"
	if !strings.HasPrefix(out.String(), want) {
		t.Errorf("wanted the header %q, got %q", want, firstLine(out.String()))
	}
}

// TestReportListsEverySiteWhenAsked asserts that --all reaches the sites a
// bucket's one example hides. A bucket whose phrase is ambiguous in English is
// settled by reading its sites, and this is how a reader reaches them.
func TestReportListsEverySiteWhenAsked(t *testing.T) {
	result := Result{
		Replacements: []Replacement{
			{File: "b.go", Line: 4, Preceding: "live", New: "column", Excerpt: "no live columns"},
			{File: "c.go", Line: 5, Preceding: "live", New: "column", Excerpt: "live column a caller would act on"},
		},
	}
	var out strings.Builder
	if err := Report(&out, result, "state", "column", true); err != nil {
		t.Fatalf("Report: %v", err)
	}
	if !strings.Contains(out.String(), "c.go:5") {
		t.Errorf("wanted every site listed, got:\n%s", out.String())
	}
}

// TestAnExcerptIsElidedAroundTheReplacement asserts that a long line reaches
// the report as one line of it, centred on the replacement. An excerpt that
// truncates from the left of the line shows a reader the indentation of the
// site rather than the phrase they have to judge.
func TestAnExcerptIsElidedAroundTheReplacement(t *testing.T) {
	filler := strings.Repeat("padding ", 20)
	line := diffLine{number: 1, text: filler + "the pending column carries " + filler}
	tokens := tokenize(line)
	var target token
	for _, tok := range tokens {
		if tok.text == "column" {
			target = tok
		}
	}
	got := excerpt(target)
	if !strings.Contains(got, "pending column") {
		t.Errorf("wanted the phrase kept, got %q", got)
	}
	if len(got) > excerptWidth+8 {
		t.Errorf("wanted an excerpt near %d bytes, got %d: %q", excerptWidth, len(got), got)
	}
}

// firstLine returns the first line of a report, which is what a failure
// message wants when the whole report would bury it.
func firstLine(text string) string {
	if at := strings.IndexByte(text, '\n'); at >= 0 {
		return text[:at]
	}
	return text
}
