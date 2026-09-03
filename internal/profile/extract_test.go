package profile

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// profilePath is the published core profile this package guards.
const profilePath = "../../docs/spec/core-profile.md"

// tradeTerms is the profile's first excluded list: the vocabulary of one
// trade, whose presence in a normative statement would tell every workbench
// outside that trade the profile was not written for it. The word "release"
// is deliberately absent, because it names one of the profile's own verbs.
var tradeTerms = []string{
	"repository", "branch", "commit", "build", "deploy", "merge",
	"pull request", "code", "bug", "ticket", "sprint", "backlog grooming",
	"continuous integration", "issue", "test suite", "developer", "engineer",
}

// productTerms is the profile's second excluded list: the product vocabulary
// of tools that already implement work like this one, which a reader of the
// profile alone has no way to interpret.
//
// `column` left this list at profile 0.7, which is the revision that made it
// one of the profile's own words. The list bars a word that arrives carrying a
// meaning the document never states; section 4 now states this one, so it no
// longer qualifies, and section 3.5 records the removal in the document
// itself rather than leaving this file as the only place it is written down.
var productTerms = []string{
	"lane", "gate", "loop limit", "station", "swimlane", "zone",
	"persona", "capability tier", "shopping queue", "external wait",
	"workstream",
}

func readProfile(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.FromSlash(profilePath))
	if err != nil {
		t.Fatalf("reading the profile: %v", err)
	}
	return string(b)
}

func extractProfile(t *testing.T) *Document {
	t.Helper()
	doc, err := Extract(strings.NewReader(readProfile(t)))
	if err != nil {
		t.Fatalf("extracting the profile: %v", err)
	}
	return doc
}

// TestExtractReadsAStatement is the unit check on the extractor itself,
// independent of any published document.
func TestExtractReadsAStatement(t *testing.T) {
	src := "prose without a keyword\n" +
		"[CORE-CLAIM-3] A tool MUST refuse a claim on a held card.\n" +
		"```\nMUST inside a fence is vocabulary\n```\n" +
		"[ACTOR-1] An owner MAY rest.\n"

	doc, err := Extract(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(doc.Statements) != 2 {
		t.Fatalf("got %d statements, want 2", len(doc.Statements))
	}
	first := doc.Statements[0]
	if first.ID != "CORE-CLAIM-3" || first.Keyword != "MUST" || first.Class != "CORE" || first.Line != 2 {
		t.Errorf("first statement read as %+v", first)
	}
	if got := doc.Statements[1].Class; got != "ACTOR" {
		t.Errorf("second statement class %q, want ACTOR", got)
	}
	if len(doc.StrayKeywords) != 0 {
		t.Errorf("a keyword inside a fence was reported stray: %v", doc.StrayKeywords)
	}
}

// TestExtractReportsDefects arms the extractor: each fault it exists to catch
// is fed to it deliberately and has to come back.
func TestExtractReportsDefects(t *testing.T) {
	src := "A stray MUST in prose.\n" +
		"[CORE-A-1] A tool MUST and also MAY.\n" +
		"[CORE-A-1] A tool MUST be named once.\n"

	doc, err := Extract(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(doc.StrayKeywords) != 1 {
		t.Errorf("got %d stray keywords, want 1", len(doc.StrayKeywords))
	}
	if len(doc.KeywordCounts) != 1 {
		t.Errorf("got %d keyword-count defects, want 1", len(doc.KeywordCounts))
	}
	if len(doc.Duplicates) != 1 {
		t.Errorf("got %d duplicates, want 1", len(doc.Duplicates))
	}
	if len(doc.Fences) != 0 {
		t.Errorf("a document with no fence at all reported %d fence defects", len(doc.Fences))
	}
}

// TestUnclosedFenceIsADefect arms the fence check. A fence that never closes
// turns keyword detection off for the rest of the document, so the extractor
// reports the fence rather than reporting the document clean.
func TestUnclosedFenceIsADefect(t *testing.T) {
	src := "[CORE-A-1] A tool MUST do the thing.\n" +
		"```\n" +
		"A stray MUST hidden under an open fence.\n"

	doc, err := Extract(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(doc.Fences) != 1 {
		t.Fatalf("got %d fence defects, want 1", len(doc.Fences))
	}
	if doc.Fences[0].Line != 2 {
		t.Errorf("fence defect reported at line %d, want 2", doc.Fences[0].Line)
	}

	closed := src + "```\n"
	doc, err = Extract(strings.NewReader(closed))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(doc.Fences) != 0 {
		t.Errorf("a closed fence was reported as a defect: %v", doc.Fences)
	}
}

// TestExcludedTermsSpanLineBreaks arms the whitespace normalization. A banned
// two-word phrase used to escape the check whenever the paragraph wrapped
// between its words, which made the width of a line decide whether the rule
// applied.
func TestExcludedTermsSpanLineBreaks(t *testing.T) {
	wrapped := "the florist's pull\nrequest was declined"
	if hits := Excluded(wrapped, []string{"pull request"}); len(hits) != 1 {
		t.Errorf("a banned phrase split by a line break was not found: %v", hits)
	}
	if hits := Excluded("the pull   request", []string{"pull request"}); len(hits) != 1 {
		t.Errorf("a banned phrase split by several spaces was not found: %v", hits)
	}
	if hits := Excluded("nothing to see", []string{"pull request"}); len(hits) != 0 {
		t.Errorf("a phrase that is absent was reported present: %v", hits)
	}
	if hits := Excluded("she pulls requests from the queue", []string{"pull request"}); len(hits) != 0 {
		t.Errorf("whole-word matching was lost in normalization: %v", hits)
	}
}

// TestNegatedKeywordCountsOnce checks the longest-first rule the notation
// section states, which is what keeps a negated keyword from reading as two.
func TestNegatedKeywordCountsOnce(t *testing.T) {
	doc, err := Extract(strings.NewReader("[CORE-A-1] A tool MUST NOT do it.\n"))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(doc.KeywordCounts) != 0 {
		t.Fatalf("MUST NOT counted as more than one keyword: %v", doc.KeywordCounts)
	}
	if got := doc.Statements[0].Keyword; got != "MUST NOT" {
		t.Errorf("keyword %q, want MUST NOT", got)
	}
}

// TestProfileExtractsCleanly asserts the profile's own notation rules over
// the published document: unique identifiers, one keyword each, and no
// keyword loose in prose.
func TestProfileExtractsCleanly(t *testing.T) {
	doc := extractProfile(t)

	if len(doc.Statements) == 0 {
		t.Fatal("the profile returned no statements")
	}
	for _, d := range doc.Duplicates {
		t.Errorf("duplicate identifier: %s", d)
	}
	for _, d := range doc.KeywordCounts {
		t.Errorf("keyword count: %s", d)
	}
	for _, d := range doc.StrayKeywords {
		t.Errorf("stray keyword: %s", d)
	}
	for _, d := range doc.Fences {
		t.Errorf("unterminated fence: %s", d)
	}
	for _, s := range doc.Statements {
		switch s.Class {
		case "CORE", "DOC", "ACTOR", "SUITE":
		default:
			t.Errorf("line %d: identifier %s carries an unknown class", s.Line, s.ID)
		}
	}
}

// TestExcludedTermsAreAbsent checks the two excluded lists over the three
// places the profile's notation section binds them to.
func TestExcludedTermsAreAbsent(t *testing.T) {
	text := readProfile(t)
	doc := extractProfile(t)

	both := append(append([]string{}, tradeTerms...), productTerms...)

	for _, s := range doc.Statements {
		if hits := Excluded(s.Text, both); len(hits) > 0 {
			t.Errorf("line %d: %s carries excluded terms %v", s.Line, s.ID, hits)
		}
	}

	vocab := section(text, "## 4. Core vocabulary")
	if vocab == "" {
		t.Fatal("section 4 not found")
	}
	if hits := Excluded(vocab, both); len(hits) > 0 {
		t.Errorf("the core vocabulary carries excluded terms %v", hits)
	}

	walk := section(text, "### 10.1 Walking a wedding through the whole profile")
	if walk == "" {
		t.Fatal("section 10.1 not found")
	}
	if hits := Excluded(walk, both); len(hits) > 0 {
		t.Errorf("the walkthrough carries excluded terms %v", hits)
	}
}

// TestIndexMatchesTheExtraction asserts that the index of section 11 carries
// one row per extracted identifier, in document order, with the keyword the
// statement itself carries.
func TestIndexMatchesTheExtraction(t *testing.T) {
	text := readProfile(t)
	doc := extractProfile(t)

	index := section(text, "## 11. Index of normative statements")
	if index == "" {
		t.Fatal("section 11 not found")
	}

	row := regexp.MustCompile(`^\| ([A-Z][A-Z0-9-]*) \| (must not|should not|must|should|may) \| (tool|document|history|suite) \| (.+) \|$`)
	var ids, keywords []string
	for _, line := range strings.Split(index, "\n") {
		m := row.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		ids = append(ids, m[1])
		keywords = append(keywords, m[2])
	}

	if len(ids) != len(doc.Statements) {
		t.Fatalf("the index carries %d rows and the extraction returns %d statements", len(ids), len(doc.Statements))
	}
	for i, s := range doc.Statements {
		if ids[i] != s.ID {
			t.Errorf("index row %d names %s, the extraction returns %s", i+1, ids[i], s.ID)
			continue
		}
		if want := strings.ToLower(s.Keyword); keywords[i] != want {
			t.Errorf("index row for %s carries keyword %q, the statement carries %q", s.ID, keywords[i], want)
		}
	}
}

// TestBoundaryTableRulesEveryStatement asserts the four properties section 10
// claims for its table: a row rules its concept in or out and carries a
// reason, an excluded concept carries a reopen condition and no statement, an
// included concept carries statements and no reopen condition, and the
// statements named across the ruled-in rows are exactly the statements the
// document publishes, each named once.
func TestBoundaryTableRulesEveryStatement(t *testing.T) {
	text := readProfile(t)
	doc := extractProfile(t)

	rows := BoundaryTable(text)
	if len(rows) == 0 {
		t.Fatal("the boundary table returned no rows")
	}

	claimed := map[string]int{}
	for _, r := range rows {
		if r.Reason == "" {
			t.Errorf("line %d: row %q carries no reason", r.Line, r.Item)
		}
		switch r.Ruling {
		case "out":
			if r.Reopen == "" {
				t.Errorf("line %d: row %q is ruled out with no reopen condition", r.Line, r.Item)
			}
			if len(r.Statements) > 0 {
				t.Errorf("line %d: row %q is ruled out but names statements %v", r.Line, r.Item, r.Statements)
			}
		case "in":
			if r.Reopen != "" {
				t.Errorf("line %d: row %q is ruled in but carries a reopen condition", r.Line, r.Item)
			}
			if len(r.Statements) == 0 {
				t.Errorf("line %d: row %q is ruled in but names no statement", r.Line, r.Item)
			}
			for _, id := range r.Statements {
				if prior, ok := claimed[id]; ok {
					t.Errorf("line %d: %s is claimed by two rows, the earlier at line %d", r.Line, id, prior)
				}
				claimed[id] = r.Line
			}
		}
	}

	published := map[string]bool{}
	for _, s := range doc.Statements {
		published[s.ID] = true
		if _, ok := claimed[s.ID]; !ok {
			t.Errorf("line %d: %s rests on a concept no row rules in", s.Line, s.ID)
		}
	}
	for id, line := range claimed {
		if !published[id] {
			t.Errorf("line %d: the boundary table names %s, which the document does not publish", line, id)
		}
	}
}

// TestBoundaryTableCatchesADefect arms the boundary check against each fault
// it exists to catch, since the published table passing says nothing about
// whether the check would notice if it stopped.
func TestBoundaryTableCatchesADefect(t *testing.T) {
	cases := map[string]string{
		"an exclusion with no reopen condition": "| A thing | out | Because. | | |\n",
		"an inclusion naming no statement":      "| A thing | in | Because. | | |\n",
		"a row with no reason":                  "| A thing | in |  | | CORE-A-1 |\n",
	}
	for name, row := range cases {
		rows := BoundaryTable(row)
		if len(rows) != 1 {
			t.Fatalf("%s: parsed %d rows, want 1", name, len(rows))
		}
		r := rows[0]
		var caught bool
		switch {
		case r.Reason == "":
			caught = true
		case r.Ruling == "out" && (r.Reopen == "" || len(r.Statements) > 0):
			caught = true
		case r.Ruling == "in" && (r.Reopen != "" || len(r.Statements) == 0):
			caught = true
		}
		if !caught {
			t.Errorf("%s: the row parsed as well formed", name)
		}
	}

	good := BoundaryTable("| A thing | in | Because. | | CORE-A-1, CORE-A-2 |\n")
	if len(good) != 1 || len(good[0].Statements) != 2 || good[0].Reopen != "" {
		t.Errorf("a well-formed row parsed as %+v", good)
	}
}

// boundaryTally matches the sentence section 10 carries beneath its table,
// stating how many rows the table holds. That sentence is a derived reference
// of the same class as a line-number-keyed fixture: somebody writes it by
// hand, adding a row falsifies it, and nothing in the document or the tool
// reads it, so it goes stale in silence.
var boundaryTally = regexp.MustCompile(`^Rows ruled in: (\d+)\. Rows ruled out: (\d+)\. Total rows: (\d+)\.$`)

// TestTheBoundaryTallyCountsTheRowsTheTableCarries holds section 10's own
// count of its rows against the rows the extraction reads.
//
// No other guard in this package looks at the sentence.
// TestBoundaryTableRulesEveryStatement reads each row's four properties and
// never counts the rows, so a row added with a correct ruling, reason and
// reopen condition leaves that check green and the sentence beside the table
// false. dinah-207 added such a row and the tally stood at the old numbers
// through a whole implementation pass.
func TestTheBoundaryTallyCountsTheRowsTheTableCarries(t *testing.T) {
	text := readProfile(t)
	rows := BoundaryTable(text)
	if len(rows) == 0 {
		t.Fatal("the boundary table returned no rows")
	}

	var in, out int
	for _, r := range rows {
		switch r.Ruling {
		case "in":
			in++
		case "out":
			out++
		}
	}
	want := []int{in, out, in + out}
	names := []string{"rows ruled in", "rows ruled out", "total rows"}

	var found int
	for i, line := range strings.Split(text, "\n") {
		m := boundaryTally.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil {
			continue
		}
		found++
		if found > 1 {
			t.Errorf("line %d: the document states its row count a second time, so two sentences now go stale independently", i+1)
			continue
		}
		for k, name := range names {
			got, err := strconv.Atoi(m[k+1])
			if err != nil {
				t.Fatalf("line %d: reading the count of %s: %v", i+1, name, err)
			}
			if got != want[k] {
				t.Errorf("line %d: the tally counts %s as %d, the table carries %d", i+1, name, got, want[k])
			}
		}
	}
	if found == 0 {
		t.Fatal("section 10 states no row count, so either the sentence was removed or its wording drifted out of the shape this guard reads")
	}
}

// indexTally matches the sentence section 11 carries beneath its index,
// stating how many rows the index holds. It is a hand-written derived figure
// of the same class as the boundary tally above, and it had already gone
// stale in the same way: at profile 0.9 it read 127 while the index carried
// 130, and nothing in the document or the tool read it, so the three-row gap
// stood through every pass over the file.
var indexTally = regexp.MustCompile(`^The index carries (\d+) rows, `)

// TestTheIndexCountCountsTheStatementsTheDocumentPublishes holds section 11's
// own count of its rows against the statements the extraction returns.
//
// TestIndexMatchesTheExtraction holds the index TABLE to the extraction and
// never reads this sentence, so a statement published with its index row in
// the right place leaves that check green and the sentence beneath the table
// false. That is how the 127 survived.
func TestTheIndexCountCountsTheStatementsTheDocumentPublishes(t *testing.T) {
	text := readProfile(t)
	doc := extractProfile(t)
	if len(doc.Statements) == 0 {
		t.Fatal("the extraction returned no statements")
	}

	var found int
	for i, line := range strings.Split(text, "\n") {
		m := indexTally.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil {
			continue
		}
		found++
		if found > 1 {
			t.Errorf("line %d: the document states its index count a second time, so two sentences now go stale independently", i+1)
			continue
		}
		got, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("line %d: reading the index count: %v", i+1, err)
		}
		if got != len(doc.Statements) {
			t.Errorf("line %d: the sentence counts the index at %d rows, the extraction returns %d statements", i+1, got, len(doc.Statements))
		}
	}
	if found == 0 {
		t.Fatal("section 11 states no row count, so either the sentence was removed or its wording drifted out of the shape this guard reads")
	}
}

// numberWords spells the counts the profile writes out in prose. A small
// count is written as a word rather than as a numeral throughout the
// document, so a guard reading such a sentence has to spell the count it
// computed before it can compare the two.
var numberWords = []string{
	"zero", "one", "two", "three", "four", "five", "six", "seven", "eight",
	"nine", "ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen",
	"sixteen", "seventeen", "eighteen", "nineteen", "twenty",
	"twenty-one", "twenty-two", "twenty-three", "twenty-four", "twenty-five",
	"twenty-six", "twenty-seven", "twenty-eight", "twenty-nine", "thirty",
}

// refusalNamesLead is the sentence section 6.1 puts ahead of the block of
// refusal names it declares. The block is located from this sentence rather
// than by taking the section's first fence, because the section carries a
// second fenced block after it.
const refusalNamesLead = "The refusal names this profile declares are these"

// refusalNameTally matches the sentence beneath that block, which states how many
// names the block holds. Same class as the two tallies above: written by
// hand, falsified by declaring a name, and read by nothing until now.
var refusalNameTally = regexp.MustCompile(`^One of the ([a-z-]+) is general where the others are particular\.`)

// TestTheRefusalTallyCountsTheNamesTheBlockDeclares holds section 6.1's count
// of the refusal names it declares against the names the block itself
// carries.
func TestTheRefusalTallyCountsTheNamesTheBlockDeclares(t *testing.T) {
	body := section(readProfile(t), "### 6.1 Outcomes and refusals")
	if body == "" {
		t.Fatal("section 6.1 not found")
	}
	lines := strings.Split(body, "\n")

	lead := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), refusalNamesLead) {
			lead = i
			break
		}
	}
	if lead < 0 {
		t.Fatalf("section 6.1 carries no sentence opening %q, so the block of refusal names can no longer be located", refusalNamesLead)
	}

	open, close := -1, -1
	for i := lead; i < len(lines); i++ {
		if !strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
			continue
		}
		if open < 0 {
			open = i
			continue
		}
		close = i
		break
	}
	if open < 0 || close < 0 {
		t.Fatal("section 6.1 carries no fenced block of refusal names after the sentence that introduces it")
	}

	var names []string
	for _, field := range strings.Split(strings.Join(lines[open+1:close], " "), ",") {
		if name := strings.TrimSpace(field); name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		t.Fatal("the block of refusal names is empty")
	}
	if len(names) >= len(numberWords) {
		t.Fatalf("the block declares %d refusal names, which numberWords cannot spell, so extend that table", len(names))
	}
	want := numberWords[len(names)]

	var found int
	for i, line := range lines {
		m := refusalNameTally.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil {
			continue
		}
		found++
		if found > 1 {
			t.Errorf("section 6.1 line %d: the section states its refusal count a second time, so two sentences now go stale independently", i+1)
			continue
		}
		if m[1] != want {
			t.Errorf("section 6.1 line %d: the sentence counts the declared refusal names as %q, the block carries %d of them (%s)", i+1, m[1], len(names), want)
		}
	}
	if found == 0 {
		t.Fatal("section 6.1 states no count of its refusal names, so either the sentence was removed or its wording drifted out of the shape this guard reads")
	}
}

// TestHouseStyle asserts the typographic rules the profile's style section
// states, which a reviewer should never have to check by eye.
//
// The banned set below is copied in the bannedTypography variable in
// cmd/dinah/guide_guard_test.go, where the check over the guides and the quick
// start reads it, so both corpora answer to one set of rules stated twice. A
// test in one package cannot read a test in another, and exporting the set from
// a shipped package would put style vocabulary in the binary, so a character
// added here is added there by hand.
func TestHouseStyle(t *testing.T) {
	text := readProfile(t)
	banned := map[string]string{
		"—": "em-dash",
		"–": "en-dash",
		"−": "minus sign",
		"‘": "left single quotation mark",
		"’": "right single quotation mark",
		"“": "left double quotation mark",
		"”": "right double quotation mark",
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "\r") {
			t.Errorf("carriage return in %q", line)
		}
		for r, name := range banned {
			if strings.Contains(line, r) {
				t.Errorf("%s in %q", name, line)
			}
		}
	}
}

// pointerPath reads a repository-relative Markdown path out of section 5.3's
// pointer to a document outside the profile. The pattern matches any such
// path rather than the one the document names today, so a sentence rewritten
// to name a different document is stat'd as written.
var pointerPath = regexp.MustCompile("`(docs/[A-Za-z0-9._/-]+\\.md)`")

// TestTheCardSectionPointsAtADocumentTheRepositoryCarries asserts both halves
// of the pointer section 5.3 carries: that the section still names a document
// outside the profile by its path, and that the working tree carries a file
// at the path the section names.
//
// The pointer exists because 5.3 opens by requiring four things of a card and
// no more, and a reader who stops there leaves believing that is the whole
// model. Deleting the sentence restores the belief, and moving the document
// without the sentence leaves a reader following a path to nothing, so both
// halves fail here rather than in front of a reader.
//
// The second half stats what the first half read instead of a constant of its
// own. A guard holding one path twice, once against the section and once
// against the filesystem, passes whenever the sentence is rewritten to name
// some other document, which is most of the rot it exists to catch.
//
// cmd/dinah/guide_guard_test.go carries the sibling check over the guides and
// the quick start. That one matches published URLs rather than a bare
// repository-relative path, so it does not reach the profile.
func TestTheCardSectionPointsAtADocumentTheRepositoryCarries(t *testing.T) {
	cards := section(readProfile(t), "### 5.3 Cards")
	m := pointerPath.FindStringSubmatch(cards)
	if m == nil {
		t.Fatal("section 5.3 names no document outside the profile. It is the section that says a card carries four things and no more, so a reader who leaves it without a pointer leaves believing that is the whole card model. Move the sentence rather than deleting it.")
	}

	named := m[1]
	if _, err := os.Stat(filepath.Join("..", "..", filepath.FromSlash(named))); err != nil {
		t.Fatalf("section 5.3 names %s and the repository carries no file there: %v. Point the sentence at wherever that document moved to.", named, err)
	}
}

// section returns the text of the document from the given heading up to the
// next heading at the same level or shallower, exclusive.
func section(text, heading string) string {
	lines := strings.Split(text, "\n")
	depth := len(heading) - len(strings.TrimLeft(heading, "#"))
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == heading {
			start = i + 1
			continue
		}
		if start < 0 || !strings.HasPrefix(line, "#") {
			continue
		}
		if d := len(line) - len(strings.TrimLeft(line, "#")); d <= depth {
			return strings.Join(lines[start:i], "\n")
		}
	}
	if start < 0 {
		return ""
	}
	return strings.Join(lines[start:], "\n")
}
