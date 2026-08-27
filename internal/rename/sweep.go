// Package rename finds the places where a word-for-word rename put the new
// word into a sentence that meant the old word's other sense.
//
// A rename across a whole tree replaces one spelling everywhere it appears,
// and a word that carries two senses gets replaced in both. Dinah renamed the
// board position from "state" to "column", and "state" also means the
// condition a thing is in, so sentences about the condition a checklist item
// is in came out talking about a board column. Twenty sentences went that way,
// and the first eleven were found by a person reading the diff by eye.
//
// Neither obvious instrument finds them. Searching the tree for the retired
// word finds the places the rename missed rather than the places it overshot,
// and an over-eager replacement leaves no trace of the retired word to find.
// Searching the tree for the new word finds every legitimate use as well, and
// on this tree that was 7126 hits.
//
// What works is to read the rename's own diff and group the replacements by
// the word standing in front of each one. A board column is a countable noun
// behind a determiner, so nearly every legitimate replacement follows "the",
// "a", "each" or a possessive. The other sense keeps different company:
// "pending state" became "pending column", and "the two unresolvable crash
// states" became "CrashColumns". Grouping by that preceding word turned the
// 6397 replacements of Dinah's own rename into 492 phrases, and every one of
// the twenty defects was in them.
//
// The identifier case is why [tokenize] splits a word at its internal case
// boundaries. When the replaced word sits inside CamelCase, the token in front
// of it on the line is a comment marker or a keyword, so every renamed
// identifier in the tree lands in one enormous group that nobody reads.
// Splitting the identifier makes the word in front of "Columns" be "Crash",
// which is the whole finding.
//
// The word after the replacement is part of the group's key for the same
// reason the word before it is. One word of company is not enough to tell an
// idiom from an accident: "the column of the board" and "the column it is in"
// share a determiner and do not share a verdict, so a key made of the
// determiner alone drops the second into a group of hundreds and the reader
// never reaches it. Two words of company make the key a phrase, and a phrase
// that appears three times in a tree is a phrase worth reading.
//
// The sweep refuses a search term it cannot read rather than reporting that it
// found nothing. See [Sweep].
package rename

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"
)

// Replacement is one place in a diff where the new word stands where the old
// word stood. Line and Column locate it on the post-rename side, so the site
// can be opened without applying anything.
type Replacement struct {
	// File is the path on the post-rename side of the diff.
	File string
	// Line is the 1-based line number on the post-rename side.
	Line int
	// Column is the 0-based byte offset of the new word within that line.
	Column int
	// Preceding is the nearest word standing in front of the replacement,
	// lowercased, with punctuation and separators skipped. It reaches back
	// into the diff's unchanged context when the replacement opens a run of
	// changed text, and is empty only where the diff carries no context
	// either, which is what a diff taken with no context lines looks like.
	Preceding string
	// Following is the nearest word standing after the replacement,
	// lowercased, read the same way Preceding is read and reaching forward
	// into the diff's unchanged context by the same rule. It is the second
	// half of the group's key, and it is what separates an idiom from an
	// ordinary English sentence that happens to open with the same
	// determiner.
	Following string
	// Glued reports that the replacement is a part of a longer identifier
	// rather than a word of its own, meaning the word in front of it abuts
	// it with nothing in between. The distinction decides the bucket,
	// because renaming a part of an identifier is nearly always right while
	// the sense of a word in prose is the whole question, and a bucket
	// holding both hides the prose site behind an identifier example.
	Glued bool
	// Old is the retired word as the pre-rename side spelled it, and New is
	// the replacement as the post-rename side spells it. Both keep their
	// original case and number so that a reader can see "States" became
	// "Columns" rather than being told a word matched.
	Old string
	New string
	// Excerpt is the post-rename line the replacement sits on, trimmed and
	// elided around the replacement so that one report line carries it.
	Excerpt string
}

// Bucket is the group of replacements sharing one phrase. The phrase is the
// group's key and is what a reader judges: the question a bucket asks is
// whether "<preceding> <new word> <following>" means the thing the rename was
// about.
type Bucket struct {
	Preceding string
	Following string
	Glued     bool
	Sites     []Replacement
}

// Unaligned names a run of changed text the sweep could not align, together
// with why. A run is reported rather than dropped, because a sweep that
// silently skips the hardest part of a diff reports a clean result for the
// place a defect is most likely to be hiding.
type Unaligned struct {
	File   string
	Line   int
	Reason string
}

// Result is what one sweep found: every replacement it aligned, and every run
// of changed text it could not.
type Result struct {
	Replacements []Replacement
	Unaligned    []Unaligned
}

// alignmentCap bounds the table the general alignment builds, in cells. A run
// of changed text whose two sides multiply out past it is reported as
// unaligned rather than aligned at a cost nobody bounded. The value allows
// roughly a thousand tokens on each side, which is some eighty lines of prose,
// and the fast path in alignRun takes every mechanical rename that leaves the
// token count alone, so a catalog rewritten on every line does not come near
// this.
const alignmentCap = 1_000_000

// excerptWidth is how much of a line an excerpt carries. Report writes one
// line per bucket, so the excerpt has to leave room for the count and the
// site.
const excerptWidth = 90

// Sweep reads a unified diff and reports every place old was replaced by new.
// Both words are matched case-insensitively, in the singular and in the plural
// formed by a trailing s, so a single call covers "state" becoming "column"
// and "States" becoming "Columns".
//
// The diff is read as text rather than applied, so the revisions it came from
// need not be checked out.
//
// Sweep returns an error when either word is not one the tokenizer reads as a
// single word, and the error is the reason this function reports one at all. A
// match is one token tested against the whole term, so a term the tokenizer
// splits can never match anything and the run would report zero replacements
// in zero groups. That answer is indistinguishable from a clean tree, and a
// reader holding a document that tells them to run this pass and report the
// count would file the zero as a pass. Refusing costs a message and a
// non-zero exit; answering zero costs the next rename its only instrument.
func Sweep(diff, retired, adopted string) (Result, error) {
	if err := checkTerm("old", retired); err != nil {
		return Result{}, err
	}
	if err := checkTerm("new", adopted); err != nil {
		return Result{}, err
	}
	retired = strings.ToLower(retired)
	adopted = strings.ToLower(adopted)
	var result Result
	for _, run := range parseDiff(diff) {
		result.merge(alignRun(run, retired, adopted))
	}
	return result, nil
}

// checkTerm refuses a search term the tokenizer cannot represent as one word.
// The rule is deliberately about the tokenizer rather than about a list of
// scripts or characters, because the failure it closes is general: whatever
// the tokenizer comes to split, a term it splits is a term this sweep cannot
// look for, and the honest answer is to say so before the run rather than to
// report the zero the run is guaranteed to produce. The flag name goes into
// the message because the caller reaches this through a command line and
// wants to know which of the two words to fix.
func checkTerm(flag, word string) error {
	tokens := tokenize(diffLine{number: 1, text: word})
	if len(tokens) == 0 {
		return fmt.Errorf("--%s carries no word, and a sweep needs a word to look for", flag)
	}
	if len(tokens) == 1 && isWord(tokens[0].text) && tokens[0].text == word {
		return nil
	}
	parts := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		parts = append(parts, tok.text)
	}
	return fmt.Errorf(
		"--%s %q is not one word to this sweep, which reads it as %s. A term the tokenizer splits matches nothing, so the run would report zero replacements and a reader would take that for a clean tree",
		flag,
		word,
		strings.Join(parts, " + "),
	)
}

// merge appends another result's findings to this one.
func (r *Result) merge(other Result) {
	r.Replacements = append(r.Replacements, other.Replacements...)
	r.Unaligned = append(r.Unaligned, other.Unaligned...)
}

// Buckets groups replacements by the phrase they sit in, which is the word
// before the replacement, the word after it, and whether the replacement is a
// word of its own or a part of a longer identifier. The order is rarest first,
// then alphabetical, because a legitimate replacement sits in the same handful
// of phrases hundreds of times while a wrong sense sits in a phrase that
// appears two or three times, so the tail of the distribution is where the
// reader's attention is worth spending and putting it at the top spends it
// first.
//
// The word after the replacement is in the key because the word before it is
// not enough on its own, and the tree proved that rather than an argument
// doing it. Seven over-eager renames of the plainest shape English has, an
// ordinary noun behind an ordinary determiner, were planted in this
// repository's own corpus and swept, and every one of them landed in a group
// of hundreds keyed on "the", "its" or "that". With the following word in the
// key the same seven surface in groups of one and two at the top of the
// report. A determiner says only that a noun follows; the word after the noun
// is where an idiom and a sentence part company.
//
// Splitting on Glued as well is what keeps a group answerable. "live column"
// in a comment and LiveColumns in a test name can share both neighbours and do
// not share a verdict, and a group carrying both shows the reader whichever of
// them happens to sort first.
func Buckets(reps []Replacement) []Bucket {
	type bucketKey struct {
		preceding string
		following string
		glued     bool
	}
	index := make(map[bucketKey]int)
	var buckets []Bucket
	for _, rep := range reps {
		key := bucketKey{preceding: rep.Preceding, following: rep.Following, glued: rep.Glued}
		at, seen := index[key]
		if !seen {
			index[key] = len(buckets)
			buckets = append(buckets, Bucket{Preceding: rep.Preceding, Following: rep.Following, Glued: rep.Glued})
			at = len(buckets) - 1
		}
		buckets[at].Sites = append(buckets[at].Sites, rep)
	}
	sort.SliceStable(buckets, func(i, j int) bool {
		if len(buckets[i].Sites) != len(buckets[j].Sites) {
			return len(buckets[i].Sites) < len(buckets[j].Sites)
		}
		if buckets[i].Preceding != buckets[j].Preceding {
			return buckets[i].Preceding < buckets[j].Preceding
		}
		if buckets[i].Following != buckets[j].Following {
			return buckets[i].Following < buckets[j].Following
		}
		return !buckets[i].Glued && buckets[j].Glued
	})
	return buckets
}

// Label renders a bucket as the phrase a reader judges, which is the preceding
// word followed by the adopted word as this bucket's own sites spell it. A
// bucket whose replacements are parts of an identifier spells the phrase
// closed up and says so, since "crashColumns" and "crash column" ask different
// questions and only one of them is about English.
//
// The phrase opens the report line rather than sitting in a padded field,
// because scanning a report means running an eye down its left edge, and a
// field padded to a width counts characters where a terminal counts columns.
// This report carries excerpts in every script the catalogs are written in, so
// a width here would drift on the first Devanagari site it met.
func (b Bucket) Label() string {
	preceding := b.Preceding
	if preceding == "" {
		preceding = "(nothing before)"
	}
	following := b.Following
	if following == "" {
		following = "(nothing after)"
	}
	if len(b.Sites) == 0 || b.Sites[0].New == "" {
		return preceding + " " + following
	}
	adopted := b.Sites[0].New
	if b.Glued {
		return preceding + adopted + " " + following + " (identifier)"
	}
	return preceding + " " + adopted + " " + following
}

// Report writes the sweep's findings to w: a header carrying the size of what
// follows, one line per bucket, and every unaligned run. When all is set each
// bucket is followed by all of its sites rather than by one example, which is
// what a reader wants once a bucket looks wrong.
func Report(w io.Writer, result Result, retired, adopted string, all bool) error {
	buckets := Buckets(result.Replacements)
	header := fmt.Sprintf(
		"%d replacements of %q by %q, in %d groups by surrounding phrase, rarest first\n",
		len(result.Replacements),
		retired,
		adopted,
		len(buckets),
	)
	if _, err := io.WriteString(w, header); err != nil {
		return err
	}
	for _, bucket := range buckets {
		line := fmt.Sprintf(
			"%s   %s   %s\n",
			bucket.Label(),
			sites(len(bucket.Sites)),
			siteLine(bucket.Sites[0]),
		)
		if _, err := io.WriteString(w, line); err != nil {
			return err
		}
		if !all {
			continue
		}
		for _, site := range bucket.Sites[1:] {
			if _, err := fmt.Fprintf(w, "    %s\n", siteLine(site)); err != nil {
				return err
			}
		}
	}
	for _, run := range result.Unaligned {
		refused := fmt.Sprintf("unaligned run   %s:%d   %s\n", run.File, run.Line, run.Reason)
		if _, err := io.WriteString(w, refused); err != nil {
			return err
		}
	}
	return nil
}

// sites counts a group's sites in words, so that a group holding one of them
// does not report itself in the plural.
func sites(n int) string {
	if n == 1 {
		return "1 site"
	}
	return fmt.Sprintf("%d sites", n)
}

// siteLine renders one replacement as a location and an excerpt.
func siteLine(rep Replacement) string {
	return fmt.Sprintf("%s:%d  %s", rep.File, rep.Line, rep.Excerpt)
}

// changedRun is one run of changed lines in a diff, with nothing unchanged
// inside it. A rename that reflows a hard-wrapped paragraph moves words across
// line boundaries, so the two sides are aligned as whole runs rather than line
// by line.
type changedRun struct {
	file    string
	removed []diffLine
	added   []diffLine
	// leadIn is the last word standing in front of the run on the
	// post-rename side, taken from the unchanged text the diff carries
	// around it. A hard-wrapped paragraph puts the word in front of a
	// replacement on the line above, and that line is unchanged whenever the
	// rename fitted inside one line, so without this a replacement opening a
	// run would be reported as having no company at all.
	leadIn string
	// leadOut is the first word standing after the run on the post-rename
	// side, taken from the unchanged text the diff carries after it. It is
	// leadIn's mirror and exists for the same wrapped-paragraph reason: a
	// replacement that closes a run has the word after it on the next line,
	// and that line is unchanged whenever the rename fitted inside one line.
	// Blank context lines carry no word, so the search runs on until a
	// context line carries one or the hunk ends.
	leadOut string
}

// diffLine is one line of a run together with where it sits on its own side.
type diffLine struct {
	number int
	text   string
}

// parseDiff splits a unified diff into its runs of changed lines. Only the
// file header and the hunk header are read for structure; everything else is
// classified by its first byte, which is what the format guarantees.
func parseDiff(diff string) []changedRun {
	var runs []changedRun
	var current changedRun
	file := ""
	leadIn := ""
	oldLine, newLine := 0, 0
	// awaiting is the index of the most recent run still looking for its
	// lead-out word, or -1 when none is.
	awaiting := -1
	flush := func() {
		if len(current.removed) == 0 && len(current.added) == 0 {
			return
		}
		current.file = file
		runs = append(runs, current)
		awaiting = len(runs) - 1
		leadIn = lastWord(tokenizeSide(current.added), leadIn)
		current = changedRun{leadIn: leadIn}
	}
	for _, line := range strings.Split(diff, "\n") {
		line = strings.TrimSuffix(line, "\r")
		switch {
		case strings.HasPrefix(line, "+++ "):
			flush()
			leadIn = ""
			awaiting = -1
			current = changedRun{}
			file = headerPath(strings.TrimPrefix(line, "+++ "))
		case strings.HasPrefix(line, "--- "):
			flush()
		case strings.HasPrefix(line, "@@"):
			flush()
			leadIn = ""
			awaiting = -1
			current = changedRun{}
			oldLine, newLine = hunkStarts(line)
		case strings.HasPrefix(line, "-"):
			current.removed = append(current.removed, diffLine{number: oldLine, text: line[1:]})
			oldLine++
		case strings.HasPrefix(line, "+"):
			current.added = append(current.added, diffLine{number: newLine, text: line[1:]})
			newLine++
		case strings.HasPrefix(line, " "):
			flush()
			context := diffLine{number: newLine, text: line[1:]}
			tokens := tokenize(context)
			if awaiting >= 0 {
				if word := firstWord(tokens, ""); word != "" {
					runs[awaiting].leadOut = word
					awaiting = -1
				}
			}
			leadIn = lastWord(tokens, leadIn)
			current = changedRun{leadIn: leadIn}
			oldLine++
			newLine++
		default:
			// A blank line, a "\ No newline at end of file" marker, a
			// commit header, or anything else the caller passed through.
			// None of them carry changed text, and none of them advance a
			// line counter.
			flush()
		}
	}
	flush()
	return runs
}

// lastWord returns the last word in a token sequence, falling back to the word
// carried in when the sequence holds none. Punctuation is skipped for the
// reason precedingWord skips it: what a reader judges is the company a word
// keeps, and a backtick is not company.
func lastWord(tokens []token, carried string) string {
	for i := len(tokens) - 1; i >= 0; i-- {
		if isWord(tokens[i].text) {
			return strings.ToLower(tokens[i].text)
		}
	}
	return carried
}

// firstWord returns the first word in a token sequence, falling back to the
// word carried in when the sequence holds none. It is lastWord read the other
// way round, and it skips punctuation for the same reason.
func firstWord(tokens []token, carried string) string {
	for _, tok := range tokens {
		if isWord(tok.text) {
			return strings.ToLower(tok.text)
		}
	}
	return carried
}

// headerPath reads the path out of a unified diff's file header, dropping the
// b/ prefix git writes and the trailing tab and timestamp the format allows.
func headerPath(rest string) string {
	if tab := strings.IndexByte(rest, '\t'); tab >= 0 {
		rest = rest[:tab]
	}
	rest = strings.TrimSpace(rest)
	if trimmed := strings.TrimPrefix(rest, "b/"); trimmed != rest {
		return trimmed
	}
	return rest
}

// hunkStarts reads the two starting line numbers out of a hunk header of the
// form @@ -12,3 +14,4 @@. A header this cannot read yields zeroes, which
// leaves the run's line numbers wrong rather than dropping its text, because a
// site reported at the wrong line is still a site a reader can search for.
func hunkStarts(header string) (int, int) {
	fields := strings.Fields(header)
	if len(fields) < 3 {
		return 0, 0
	}
	return startOf(fields[1], '-'), startOf(fields[2], '+')
}

// startOf reads the line number out of one half of a hunk header, such as
// -12,3 or +14.
func startOf(field string, sign byte) int {
	if len(field) == 0 || field[0] != sign {
		return 0
	}
	field = field[1:]
	if comma := strings.IndexByte(field, ','); comma >= 0 {
		field = field[:comma]
	}
	number := 0
	for i := 0; i < len(field); i++ {
		if field[i] < '0' || field[i] > '9' {
			return 0
		}
		number = number*10 + int(field[i]-'0')
	}
	return number
}

// token is one lexical piece of a side of a run, carrying where it came from
// so that a replacement can be reported at its own line and offset.
type token struct {
	text   string
	line   int
	column int
	source string
}

// tokenize splits a line into tokens. A run of letters breaks at each internal
// case boundary, so CamelCase and an all-caps prefix both come apart into the
// words a person reads them as; a run of digits is one token; and every other
// non-space byte stands alone. Whitespace produces nothing.
//
// The case split is what makes an identifier legible to the sweep. Without it
// the word standing in front of the replacement inside FooStates is whatever
// began the line, and every identifier in the tree lands in one bucket.
func tokenize(line diffLine) []token {
	var tokens []token
	runes := []rune(line.text)
	offsets := make([]int, len(runes)+1)
	at := 0
	for i, r := range runes {
		offsets[i] = at
		at += len(string(r))
	}
	offsets[len(runes)] = at
	emit := func(from, to int) {
		tokens = append(tokens, token{
			text:   string(runes[from:to]),
			line:   line.number,
			column: offsets[from],
			source: line.text,
		})
	}
	for i := 0; i < len(runes); {
		r := runes[i]
		switch {
		case unicode.IsSpace(r):
			i++
		case unicode.IsLetter(r):
			end := wordEnd(runes, i)
			emit(i, end)
			i = end
		case unicode.IsDigit(r):
			end := i
			for end < len(runes) && unicode.IsDigit(runes[end]) {
				end++
			}
			emit(i, end)
			i = end
		default:
			emit(i, i+1)
			i++
		}
	}
	return tokens
}

// zeroWidthNonJoiner and zeroWidthJoiner sit inside a word rather than between
// two. Indic scripts spell a broken conjunct with the first and a joined one
// with the second, and neither is a boundary a reader sees.
const (
	zeroWidthNonJoiner = '‌'
	zeroWidthJoiner    = '‍'
)

// isCombining reports whether a rune carries on a word already begun without
// being a letter itself. Every combining mark does, and so do the two
// zero-width joiners.
//
// This exists because a script that writes its vowels as marks on a consonant
// spells one word out of runes that unicode.IsLetter refuses. Hindi कॉलम
// carries the vowel sign U+0949 and स्तंभ carries the virama U+094D and the
// anusvara U+0902, so a tokenizer that breaks at every non-letter shreds both
// into single consonants and no token can ever equal the word a caller is
// looking for. Dinah's own Hindi rename swept to zero replacements in zero
// groups until this existed, over the range that carried all ninety-nine of
// them.
func isCombining(r rune) bool {
	if r == zeroWidthNonJoiner || r == zeroWidthJoiner {
		return true
	}
	return unicode.In(r, unicode.Mn, unicode.Mc, unicode.Me)
}

// wordEnd returns the index just past the word beginning at start, breaking at
// an internal case boundary and carrying on across a combining mark. A
// lowercase or title-case letter followed by an uppercase one ends the word,
// as in cardState, and a run of uppercase letters ends one letter early when a
// lowercase letter follows, so HTTPState comes apart as HTTP and State rather
// than HTTPS and tate.
func wordEnd(runes []rune, start int) int {
	i := start + 1
	for i < len(runes) && (unicode.IsLetter(runes[i]) || isCombining(runes[i])) {
		previous := runes[i-1]
		current := runes[i]
		if !unicode.IsUpper(previous) && unicode.IsUpper(current) {
			return i
		}
		if unicode.IsUpper(previous) && unicode.IsUpper(current) {
			if i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
				return i
			}
		}
		i++
	}
	return i
}

// tokenizeSide flattens one side of a run into a single token sequence.
func tokenizeSide(lines []diffLine) []token {
	var tokens []token
	for _, line := range lines {
		tokens = append(tokens, tokenize(line)...)
	}
	return tokens
}

// matchesWord reports whether a token is the given word, in either number and
// in any case. The plural is the trailing s, which is the only inflection an
// English noun rename reliably produces and the only one the sweep claims to
// cover; a rename between words with an irregular plural is run twice.
func matchesWord(text, word string) bool {
	lowered := strings.ToLower(text)
	return lowered == word || lowered == word+"s"
}

// alignRun finds the replacements in one run of changed lines.
//
// The fast path is the whole point of the structure. A mechanical rename
// leaves the token count alone and changes nothing but the renamed word, so
// the two sides line up index for index and no alignment is needed. That
// covers the bulk of any rename, including a catalog whose every line changed.
// The general path runs only where the run carries other edits as well, which
// is where prose was rewrapped or a sentence rewritten, and those runs are
// small.
func alignRun(run changedRun, retired, adopted string) Result {
	before := tokenizeSide(run.removed)
	after := tokenizeSide(run.added)
	if len(before) == 0 || len(after) == 0 {
		return Result{}
	}
	if pairs, ok := alignByIndex(before, after, retired, adopted); ok {
		return Result{Replacements: report(run, before, after, pairs)}
	}
	if len(before)*len(after) > alignmentCap {
		reason := fmt.Sprintf(
			"%d changed tokens against %d is past the alignment cap of %d cells, so this run was not swept",
			len(before),
			len(after),
			alignmentCap,
		)
		return Result{Unaligned: []Unaligned{{File: run.file, Line: run.added[0].number, Reason: reason}}}
	}
	pairs := alignByEdits(before, after, retired, adopted)
	return Result{Replacements: report(run, before, after, pairs)}
}

// pairing is one replacement expressed as indices into the two token sides.
type pairing struct {
	before int
	after  int
}

// alignByIndex pairs two sides that differ only where the rename touched them.
// It reports false as soon as it meets a difference the rename does not
// explain, which sends the run to the general alignment rather than letting a
// coincidence of length pass for an alignment.
func alignByIndex(before, after []token, retired, adopted string) ([]pairing, bool) {
	if len(before) != len(after) {
		return nil, false
	}
	var pairs []pairing
	for i := range before {
		if before[i].text == after[i].text {
			continue
		}
		if !matchesWord(before[i].text, retired) || !matchesWord(after[i].text, adopted) {
			return nil, false
		}
		pairs = append(pairs, pairing{before: i, after: i})
	}
	return pairs, true
}

// alignByEdits pairs the replacements in a run whose two sides differ by more
// than the rename. It takes the longest common subsequence of the two token
// sequences, walks the edits that lie between the common tokens, and pairs the
// retired words deleted in one edit with the new words inserted in the same
// edit, in order. Pairing inside the edit rather than across the run is what
// keeps a sentence that was rewritten from claiming a replacement that never
// happened.
func alignByEdits(before, after []token, retired, adopted string) []pairing {
	table := longestCommon(before, after)
	var pairs []pairing
	i, j := 0, 0
	width := len(after) + 1
	for i < len(before) && j < len(after) {
		if before[i].text == after[j].text {
			i++
			j++
			continue
		}
		startI, startJ := i, j
		for i < len(before) && j < len(after) && before[i].text != after[j].text {
			if table[(i+1)*width+j] >= table[i*width+j+1] {
				i++
			} else {
				j++
			}
		}
		pairs = append(pairs, pairEdit(before[startI:i], after[startJ:j], startI, startJ, retired, adopted)...)
	}
	// The loop above exits only when one side is exhausted, so whatever is
	// left is a tail on one side alone with nothing on the other to pair it
	// against. There is no replacement to be seen in a deletion nobody
	// inserted anything for, so the walk ends here.
	return pairs
}

// pairEdit pairs the retired words deleted in one edit with the new words
// inserted in it, in the order each appears. An edit deleting two retired
// words and inserting one new word pairs the first of each and leaves the
// second unpaired, which is the honest reading: only one replacement is
// visible in the text.
func pairEdit(deleted, inserted []token, offsetBefore, offsetAfter int, retired, adopted string) []pairing {
	var olds, news []int
	for i, tok := range deleted {
		if matchesWord(tok.text, retired) {
			olds = append(olds, offsetBefore+i)
		}
	}
	for j, tok := range inserted {
		if matchesWord(tok.text, adopted) {
			news = append(news, offsetAfter+j)
		}
	}
	count := len(olds)
	if len(news) < count {
		count = len(news)
	}
	pairs := make([]pairing, 0, count)
	for k := 0; k < count; k++ {
		pairs = append(pairs, pairing{before: olds[k], after: news[k]})
	}
	return pairs
}

// longestCommon builds the classic longest-common-subsequence length table for
// the two token sequences, indexed as table[i*(len(after)+1)+j]. The caller
// bounds the size before asking for it.
func longestCommon(before, after []token) []int32 {
	width := len(after) + 1
	table := make([]int32, (len(before)+1)*width)
	for i := len(before) - 1; i >= 0; i-- {
		for j := len(after) - 1; j >= 0; j-- {
			switch {
			case before[i].text == after[j].text:
				table[i*width+j] = table[(i+1)*width+j+1] + 1
			case table[(i+1)*width+j] >= table[i*width+j+1]:
				table[i*width+j] = table[(i+1)*width+j]
			default:
				table[i*width+j] = table[i*width+j+1]
			}
		}
	}
	return table
}

// report turns paired indices into replacements, reading the preceding word
// off the post-rename side so that it is the company the new word actually
// keeps rather than the company the old word kept.
func report(run changedRun, before, after []token, pairs []pairing) []Replacement {
	reps := make([]Replacement, 0, len(pairs))
	for _, pair := range pairs {
		tok := after[pair.after]
		reps = append(reps, Replacement{
			File:      run.file,
			Line:      tok.line,
			Column:    tok.column,
			Preceding: precedingWord(after, pair.after, run.leadIn),
			Following: followingWord(after, pair.after, run.leadOut),
			Glued:     glued(after, pair.after),
			Old:       before[pair.before].text,
			New:       tok.text,
			Excerpt:   excerpt(tok),
		})
	}
	return reps
}

// glued reports whether the token at the given index is a part of a longer
// identifier: the token before it is a word, sits on the same line, and ends
// exactly where this one begins, so nothing separates them in the source. A
// case boundary is the usual separator, as in liveColumn, and an underscore is
// not glue, because live_column reads as two words and its parts are judged as
// two words.
func glued(tokens []token, at int) bool {
	if at == 0 {
		return false
	}
	previous := tokens[at-1]
	if !isWord(previous.text) || previous.line != tokens[at].line {
		return false
	}
	return previous.column+len(previous.text) == tokens[at].column
}

// precedingWord returns the word standing in front of the token at the given
// index, falling back to the run's lead-in when every token before it on the
// changed side is punctuation or there are none. Punctuation is skipped so
// that a backticked or underscored occurrence buckets with its prose
// neighbours: the word in front of `state` is "the" rather than a backtick,
// and the word in front of the second half of crash_state is "crash".
func precedingWord(tokens []token, at int, leadIn string) string {
	return lastWord(tokens[:at], leadIn)
}

// followingWord returns the word standing after the token at the given index,
// falling back to the run's lead-out when every token after it on the changed
// side is punctuation or there are none. It skips punctuation for the reason
// precedingWord does, so the word after `column` is the next word rather than
// a backtick.
func followingWord(tokens []token, at int, leadOut string) string {
	return firstWord(tokens[at+1:], leadOut)
}

// isWord reports whether a token is made of letters or digits, which is what
// separates a word from the punctuation between words. A combining mark counts
// as part of a word, since that is where it belongs, but a token holding
// nothing else is a stray mark rather than a word and does not count as one.
func isWord(text string) bool {
	letters := 0
	for _, r := range text {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			letters++
		case isCombining(r):
		default:
			return false
		}
	}
	return letters > 0
}

// excerpt renders the line a replacement sits on, elided around the
// replacement so that the report keeps one line per site.
func excerpt(tok token) string {
	line := strings.TrimRight(tok.source, " \t")
	if len(line) <= excerptWidth {
		return strings.TrimLeft(line, " \t")
	}
	start := tok.column - excerptWidth/2
	if start < 0 {
		start = 0
	}
	end := start + excerptWidth
	if end > len(line) {
		end = len(line)
		start = end - excerptWidth
	}
	for start > 0 && !isBoundary(line[start]) {
		start--
	}
	for end < len(line) && !isBoundary(line[end]) {
		end++
	}
	shown := strings.TrimSpace(line[start:end])
	if start > 0 {
		shown = "..." + shown
	}
	if end < len(line) {
		shown += "..."
	}
	return shown
}

// isBoundary reports whether a byte is a safe place to cut a line, which keeps
// an elision from splitting a word or a multi-byte rune.
func isBoundary(b byte) bool {
	return b == ' ' || b == '\t'
}
