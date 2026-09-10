package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/verb"
)

// The prose figure guard, and the boundary it stops at.
//
// The replay in quickstart_test.go holds every transcript the documents show,
// and until this file existed it held not one sentence. A document saying that
// `dinah help` lists fifty-one commands was checked by nothing, which is how
// six false statements stood in the tree while the suite was green. The ledger
// at testdata/prose-figures.txt registers every figure the guard can see, and
// the five checks below hold each one to the set it counts.
//
// What this guard cannot see, written down because a boundary recorded as
// prose and acted on by nothing is the defect this file exists to answer:
//
//   - A figure standing beside a noun the ledger does not register. The reach
//     is nine nouns and 23 occurrences out of the 385 figures the corpus
//     carries, because a guard reading all 385 would fail on ordinary English
//     and be switched off within a week. A sentence saying that Dinah ships
//     nineteen refusals raises nothing here.
//   - A new countable noun entering the corpus. Nothing notices, so the
//     registry carries the same "somebody must remember" failure as the counts
//     it holds, moved one level up. What it buys is a file with a reviewable
//     diff in place of a sentence in a comment.
//   - Whether a `by=` pointer names a test that holds the figure. The check
//     proves the named test is declared somewhere in the module and nothing
//     more. It cannot tell whether that test opens the document or counts
//     anything, so the entry's reason carries that claim and a person is the
//     one who checked it. An earlier draft of this ledger pointed at a test
//     that never read the document it was cited for, which is why the class
//     says what it proves.
//   - Whether a `holds=unreachable` figure is still true. That class points at
//     the declaration of a set this package cannot reach, and the check proves
//     the named line carries the named identifier. It never compares the
//     figure against the set, so a ninth basis-consuming tool or a sixth
//     journal field leaves the sentence stale with every check here green. Two
//     of the twenty-three entries sit in that state today.
//   - Two entries naming one set in different words. The consistency rule
//     compares the `counts=` phrases as written, so a reworded phrase leaves
//     its group.
//   - Everything a sentence says that is not a count. A promise, an order of
//     explanation, and an example that teaches the wrong thing are all outside
//     what any of this reaches.

// proseFigureLedger is the file declaring every figure this guard holds.
var proseFigureLedger = filepath.Join("testdata", "prose-figures.txt")

// proseFigureKeys are the directives a figure entry may carry. Every other
// word continues the value it stands in, which is what lets `counts=` and
// `reason=` be prose.
var proseFigureKeys = []string{"figure", "noun", "counts", "derives", "holds", "at", "declares", "by", "reason"}

// proseHoldings are the classes a figure no derivation reaches may declare.
var proseHoldings = []string{"partitive", "elsewhere", "unreachable", "prose"}

// proseEntry is one line of the ledger: one figure, beside one registered
// noun, at one line of one document.
type proseEntry struct {
	// document is the file the figure stands in, named as the ledger spells
	// it, which is a slash-separated path from the repository root.
	document string
	// line is the one-based line the figure stands on.
	line int
	// figure is the number word or the digits as the document writes them.
	figure string
	// noun is the registered noun the figure stands beside.
	noun string
	// counts is the prose naming the set the figure counts.
	counts string
	// derives names a derivation in derivationsByName, and holds names one
	// of proseHoldings. An entry carries exactly one of the two.
	derives string
	holds   string
	// at and declares are the pointer of a holds=unreachable entry.
	at       string
	declares string
	// by names the test a holds=elsewhere entry rests on.
	by string
	// reason is why the figure is held the way it is.
	reason string
	// source is the one-based line of the ledger, for a finding to name.
	source int
}

// holding is how the entry says the figure is held, in the spelling the
// consistency rule compares.
func (e proseEntry) holding() string {
	if e.derives != "" {
		return "derives=" + e.derives
	}
	return "holds=" + e.holds
}

// where names the occurrence a finding is about.
func (e proseEntry) where() string {
	return fmt.Sprintf("%s:%d", e.document, e.line)
}

// proseOccurrence is one figure the scan found standing beside a registered
// noun.
type proseOccurrence struct {
	document string
	line     int
	figure   string
	noun     string
}

// proseCorpus is every document this guard reads: the guides the binary ships
// and the quick start, the latter under the path the ledger spells rather than
// the relative one the replay opens it by.
func proseCorpus(t *testing.T) []guardedDocument {
	t.Helper()
	documents := embeddedGuides(t)
	source, err := os.ReadFile(quickStartPath)
	if err != nil {
		t.Fatalf("read %s: %v", quickStartPath, err)
	}
	documents = append(documents, guardedDocument{
		name: "docs/quick-start.md",
		text: strings.ReplaceAll(string(source), "\r\n", "\n"),
	})
	return documents
}

// readProseFigures reads the ledger into its registered nouns and its entries,
// reporting a malformed line rather than skipping it. A blank line and a line
// opening with a hash are commentary.
func readProseFigures(t *testing.T) ([]string, []proseEntry) {
	t.Helper()
	source, err := os.ReadFile(proseFigureLedger)
	if err != nil {
		t.Fatalf("read %s: %v", proseFigureLedger, err)
	}
	var nouns []string
	var entries []proseEntry
	for number, line := range strings.Split(strings.ReplaceAll(string(source), "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		first, rest, _ := strings.Cut(trimmed, " ")
		if first == "noun" {
			noun := strings.TrimSpace(rest)
			if noun == "" || strings.Contains(noun, " ") {
				t.Errorf("%s:%d: a noun line registers one noun and carries nothing else, and this one carries %q", proseFigureLedger, number+1, rest)
				continue
			}
			nouns = append(nouns, noun)
			continue
		}
		entry, err := parseProseEntry(first, rest)
		if err != nil {
			t.Errorf("%s:%d: %v", proseFigureLedger, number+1, err)
			continue
		}
		entry.source = number + 1
		entries = append(entries, entry)
	}
	if len(nouns) == 0 {
		t.Fatalf("%s registers no noun, so the scan reads nothing", proseFigureLedger)
	}
	if len(entries) == 0 {
		t.Fatalf("%s declares no figure, so every check over it reads nothing", proseFigureLedger)
	}
	return nouns, entries
}

// parseProseEntry reads one figure entry, whose first word is the document and
// the line the figure stands on.
func parseProseEntry(first, rest string) (proseEntry, error) {
	entry := proseEntry{}
	document, number, ok := strings.Cut(first, ":")
	if !ok {
		return entry, fmt.Errorf("the entry opens %q, and an entry opens with the document and the line its figure stands on", first)
	}
	at, err := strconv.Atoi(number)
	if err != nil {
		return entry, fmt.Errorf("the entry names the line %q, which is not a number", number)
	}
	entry.document, entry.line = document, at
	for key, value := range splitDirectives(rest, proseFigureKeys) {
		switch key {
		case "figure":
			entry.figure = value
		case "noun":
			entry.noun = value
		case "counts":
			entry.counts = value
		case "derives":
			entry.derives = value
		case "holds":
			entry.holds = value
		case "at":
			entry.at = value
		case "declares":
			entry.declares = value
		case "by":
			entry.by = value
		case "reason":
			entry.reason = value
		}
	}
	if err := entry.validate(); err != nil {
		return entry, err
	}
	return entry, nil
}

// validate reports what a well-formed entry must carry and this one does not.
func (e proseEntry) validate() error {
	switch {
	case e.figure == "":
		return fmt.Errorf("the entry declares no figure=")
	case e.noun == "":
		return fmt.Errorf("the entry declares no noun=")
	case e.counts == "":
		return fmt.Errorf("the entry declares no counts=, and every entry names the set its figure counts")
	case e.derives != "" && e.holds != "":
		return fmt.Errorf("the entry declares both derives=%s and holds=%s, and an entry carries one of the two", e.derives, e.holds)
	case e.derives == "" && e.holds == "":
		return fmt.Errorf("the entry declares neither derives= nor holds=, so nothing says how the figure is held")
	}
	if e.derives != "" {
		return nil
	}
	if !namesADirective(e.holds, proseHoldings) {
		return fmt.Errorf("the entry holds the figure as %q, and the classes are %s", e.holds, strings.Join(proseHoldings, ", "))
	}
	switch e.holds {
	case "partitive", "prose":
		if e.reason == "" {
			return fmt.Errorf("a holds=%s entry declares no reason=, and the reason is the whole of what such an entry says", e.holds)
		}
	case "elsewhere":
		if e.by == "" || e.reason == "" {
			return fmt.Errorf("a holds=elsewhere entry names the test under by= and says under reason= how that test holds the figure")
		}
	case "unreachable":
		if e.at == "" || e.declares == "" {
			return fmt.Errorf("a holds=unreachable entry names the declaration under at=<file>:<line> and the identifier under declares=")
		}
	}
	return nil
}

// proseFigurePattern builds the scan's one pattern from the registered nouns.
//
// A figure is a number word from one to ninety-nine or a run of digits, and it
// is visible only when it stands within two words of a registered noun. The
// alternation is ordered longest first, so `twenty-one` matches as one figure
// rather than as `twenty`. The gap admits at most two intervening words, each
// alphabetic and lower case, so a match reaching across a quotation mark, a
// backtick, a comma, or a heading marker is not a match.
func proseFigurePattern(nouns []string) *regexp.Regexp {
	words := numberWords()
	sort.Slice(words, func(i, j int) bool {
		if len(words[i]) != len(words[j]) {
			return len(words[i]) > len(words[j])
		}
		return words[i] < words[j]
	})
	sorted := append([]string(nil), nouns...)
	sort.Strings(sorted)
	return regexp.MustCompile(
		`(?P<figure>(?i:` + strings.Join(words, "|") + `|[0-9]+))` +
			`(?P<gap>( [a-z]+){0,2}) ` +
			`(?P<noun>(?i:` + strings.Join(sorted, "|") + `))\b`)
}

// proseOccurrences scans every document of the corpus outside its fenced
// blocks and returns every figure standing beside a registered noun.
//
// Go's regexp carries no lookbehind, so the pattern matches the whole shape
// and a match whose figure is preceded by a word character, a backtick, or a
// hyphen is dropped here. That is what keeps `dinah-446` and `<column:one>`
// from reading as figures.
func proseOccurrences(t *testing.T, nouns []string) []proseOccurrence {
	t.Helper()
	pattern := proseFigurePattern(nouns)
	figure := pattern.SubexpIndex("figure")
	noun := pattern.SubexpIndex("noun")
	var found []proseOccurrence
	for _, document := range proseCorpus(t) {
		for at, line := range proseLinesOf(document.text) {
			if line == "" {
				continue
			}
			for _, match := range pattern.FindAllStringSubmatchIndex(line, -1) {
				start := match[2*figure]
				if start > 0 && precedesAFigure(line[start-1]) {
					continue
				}
				found = append(found, proseOccurrence{
					document: document.name,
					line:     at + 1,
					figure:   line[start:match[2*figure+1]],
					noun:     strings.ToLower(line[match[2*noun]:match[2*noun+1]]),
				})
			}
		}
	}
	return found
}

// precedesAFigure reports whether a character standing before a match makes
// that match part of a longer token rather than a figure of its own.
func precedesAFigure(b byte) bool {
	switch {
	case b >= '0' && b <= '9', b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z':
		return true
	case b == '_', b == '`', b == '-':
		return true
	}
	return false
}

// proseLinesOf returns a document's lines with every line inside a fenced
// block blanked, so the scan reads prose and no transcript. The fence
// arithmetic is quickStartMarkerRun, which is the one predicate every reading
// of a fence in this package shares.
func proseLinesOf(text string) []string {
	lines := strings.Split(text, "\n")
	prose := make([]string, len(lines))
	open := 0
	for i, line := range lines {
		run := quickStartMarkerRun(line)
		if open == 0 {
			if run != 0 {
				open = run
				continue
			}
			prose[i] = line
			continue
		}
		if run == open {
			open = 0
		}
	}
	return prose
}

// numberWords lists the spellings the scan admits, which are one to
// ninety-nine.
func numberWords() []string {
	tens := []string{"twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}
	units := []string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}
	teens := []string{"ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen"}
	words := append([]string(nil), units...)
	words = append(words, teens...)
	for _, ten := range tens {
		words = append(words, ten)
		for _, unit := range units {
			words = append(words, ten+"-"+unit)
		}
	}
	return words
}

// numberWord spells a whole number from ten to ninety-nine the way the
// documents spell one. The range is the range a command count can occupy in a
// document a reader would notice, and a number outside it returns the digits
// so a caller sees the miss rather than a plausible wrong word.
//
// The teens are spelled rather than left to the digits, because a roster
// crossing from nine to seventeen put a derived figure in that range and the
// digits are not what the guide's sentence says.
func numberWord(n int) string {
	teens := map[int]string{
		10: "ten", 11: "eleven", 12: "twelve", 13: "thirteen", 14: "fourteen",
		15: "fifteen", 16: "sixteen", 17: "seventeen", 18: "eighteen", 19: "nineteen",
	}
	if word, named := teens[n]; named {
		return word
	}
	tens := map[int]string{2: "twenty", 3: "thirty", 4: "forty", 5: "fifty", 6: "sixty", 7: "seventy", 8: "eighty", 9: "ninety"}
	units := []string{"", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}
	ten, ok := tens[n/10]
	if !ok {
		return strconv.Itoa(n)
	}
	if n%10 == 0 {
		return ten
	}
	return ten + "-" + units[n%10]
}

// numberFromWord reads a figure as a document writes it, in digits or in the
// spellings numberWords admits, and reports whether it could read it at all.
// The reading is case-insensitive, because a sentence-opening "Five" and a
// mid-sentence "five" are the same claim.
func numberFromWord(figure string) (int, bool) {
	figure = strings.ToLower(strings.TrimSpace(figure))
	if value, err := strconv.Atoi(figure); err == nil {
		return value, true
	}
	units := map[string]int{"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7, "eight": 8, "nine": 9}
	teens := map[string]int{
		"ten": 10, "eleven": 11, "twelve": 12, "thirteen": 13, "fourteen": 14,
		"fifteen": 15, "sixteen": 16, "seventeen": 17, "eighteen": 18, "nineteen": 19,
	}
	tens := map[string]int{"twenty": 20, "thirty": 30, "forty": 40, "fifty": 50, "sixty": 60, "seventy": 70, "eighty": 80, "ninety": 90}
	if value, ok := units[figure]; ok {
		return value, true
	}
	if value, ok := teens[figure]; ok {
		return value, true
	}
	ten, unit, hyphenated := strings.Cut(figure, "-")
	base, ok := tens[ten]
	if !ok {
		return 0, false
	}
	if !hyphenated {
		return base, true
	}
	added, ok := units[unit]
	if !ok {
		return 0, false
	}
	return base + added, true
}

// derivationsByName maps a derivation to the count the binary yields for it.
//
// This map is the only place a derivation is spelled, and an entry naming a
// derivation it does not carry fails. A derivation must be computable from
// this package out of exported API, the in-package commands table, or runCLI
// output, which is what makes holds=unreachable a class rather than an excuse.
// A derivation returning zero is a failure rather than a comparison, because a
// derivation that read nothing proves nothing.
var derivationsByName = map[string]func(t *testing.T) (int, error){
	"groupedCommands": func(t *testing.T) (int, error) {
		listed := 0
		for _, c := range commands {
			if c.group != "" {
				listed++
			}
		}
		return listed, nil
	},
	"commandGroups": func(t *testing.T) (int, error) {
		groups := map[string]bool{}
		for _, c := range commands {
			if c.group != "" {
				groups[c.group] = true
			}
		}
		return len(groups), nil
	},
	"contractVerbs": func(t *testing.T) (int, error) {
		return len(verb.ContractVerbs), nil
	},
	"configKeys": func(t *testing.T) (int, error) {
		return len(bench.ConfigKeys), nil
	},
	"checklistKinds": func(t *testing.T) (int, error) {
		return len(bench.ItemKinds), nil
	},
	"referenceCommands": func(t *testing.T) (int, error) {
		return len(commandsTakingAReference()), nil
	},
	"qualifiedTableRows": func(t *testing.T) (int, error) {
		qualified, _ := referencesGuideQualifiedRows(t)
		return len(qualified), nil
	},
	"workbenchSpellings": func(t *testing.T) (int, error) {
		return len(referencesGuideWorkbenchSpellings(t)), nil
	},
	"queryFields": func(t *testing.T) (int, error) {
		return len(verb.QueryFields), nil
	},
	"columnNewFlags": func(t *testing.T) (int, error) {
		flags := 0
		for _, param := range verb.Params("column") {
			if param.Flag {
				flags++
			}
		}
		return flags, nil
	},
}

// TestEveryProseFigureIsDeclared fails on a figure standing beside a
// registered noun that the ledger carries no entry for. This is the class of
// the audit's first finding, which was a figure nobody registered and
// therefore nobody re-derived.
func TestEveryProseFigureIsDeclared(t *testing.T) {
	nouns, entries := readProseFigures(t)
	occurrences := proseOccurrences(t, nouns)
	if len(occurrences) == 0 {
		t.Fatalf("the corpus carries no figure beside any of the %d registered nouns, so this check read nothing", len(nouns))
	}
	declared := map[string]bool{}
	for _, entry := range entries {
		declared[proseKey(entry.document, entry.line, entry.figure, entry.noun)] = true
	}
	for _, found := range occurrences {
		if declared[proseKey(found.document, found.line, found.figure, found.noun)] {
			continue
		}
		t.Errorf("%s:%d says %s %s and %s carries no entry for it; write one saying what the figure counts and what holds it",
			found.document, found.line, found.figure, found.noun, proseFigureLedger)
	}
}

// TestNoProseFigureEntryIsStale fails on an entry whose document and line no
// longer carry the figure and the noun it names, so a moved sentence is
// repaired rather than left pointing at a line that has become something else.
func TestNoProseFigureEntryIsStale(t *testing.T) {
	nouns, entries := readProseFigures(t)
	standing := map[string]bool{}
	for _, found := range proseOccurrences(t, nouns) {
		standing[proseKey(found.document, found.line, found.figure, found.noun)] = true
	}
	if len(standing) == 0 {
		t.Fatal("the corpus carries no figure beside any registered noun, so this check read nothing")
	}
	for _, entry := range entries {
		if standing[proseKey(entry.document, entry.line, entry.figure, entry.noun)] {
			continue
		}
		t.Errorf("%s:%d: the entry expects the figure %s beside %s at %s:%d, and that line carries no such figure",
			proseFigureLedger, entry.source, entry.figure, entry.noun, entry.document, entry.line)
	}
}

// proseKey identifies one occurrence. The figure is folded to lower case,
// because a sentence-opening "Five" and a mid-sentence "five" are one claim.
func proseKey(document string, line int, figure, noun string) string {
	return fmt.Sprintf("%s:%d:%s:%s", document, line, strings.ToLower(figure), strings.ToLower(noun))
}

// TestEveryDerivedProseFigureMatchesTheBinary runs the derivation of every
// entry carrying derives= and holds the document's figure to what the binary
// yields. An entry naming a derivation the map does not carry is left to
// TestEveryLedgerReferenceIsLive, which owns every dead reference.
func TestEveryDerivedProseFigureMatchesTheBinary(t *testing.T) {
	_, entries := readProseFigures(t)
	compared := 0
	for _, entry := range entries {
		if entry.derives == "" {
			continue
		}
		derivation, known := derivationsByName[entry.derives]
		if !known {
			continue
		}
		got, err := derivation(t)
		if err != nil {
			t.Errorf("%s:%d: the derivation %s could not be computed: %v", proseFigureLedger, entry.source, entry.derives, err)
			continue
		}
		if got == 0 {
			t.Errorf("%s:%d: the derivation %s yields zero, so it read nothing and proves nothing", proseFigureLedger, entry.source, entry.derives)
			continue
		}
		want, readable := numberFromWord(entry.figure)
		if !readable {
			t.Errorf("%s:%d: the figure %s cannot be read as a number, so nothing can hold it", entry.document, entry.line, entry.figure)
			continue
		}
		compared++
		if want != got {
			t.Errorf("%s:%d says %s %s, and the derivation %s yields %d, which the documents spell %s",
				entry.document, entry.line, entry.figure, entry.noun, entry.derives, got, numberWord(got))
		}
	}
	if compared == 0 {
		t.Fatalf("%s declares no derived figure, so this check read nothing", proseFigureLedger)
	}
}

// TestEveryCountedSetIsCountedConsistently holds the ledger to itself. Two
// entries naming the same set must carry the same figure and must be held the
// same way, which is what stops an author escaping a derivation on one of
// several sibling statements: the corpus states the five verbs seven times, so
// escaping the derivation costs a seven-line diff a reviewer can refuse.
//
// The same check refuses a counts= phrase spelled as the literal none. The
// grouping reads the phrase as written, so a shared sentinel puts unrelated
// figures in one group and reddens a run over a collision rather than over a
// disagreement. A figure that counts nothing still stands for something.
func TestEveryCountedSetIsCountedConsistently(t *testing.T) {
	_, entries := readProseFigures(t)
	grouped := map[string][]proseEntry{}
	var phrases []string
	for _, entry := range entries {
		if strings.EqualFold(strings.TrimSpace(entry.counts), "none") {
			t.Errorf("%s:%d: the entry for %s writes counts=none; write a phrase saying what the figure stands for, because a shared sentinel groups unrelated figures together",
				proseFigureLedger, entry.source, entry.where())
			continue
		}
		if _, seen := grouped[entry.counts]; !seen {
			phrases = append(phrases, entry.counts)
		}
		grouped[entry.counts] = append(grouped[entry.counts], entry)
	}
	for _, phrase := range phrases {
		group := grouped[phrase]
		if len(group) < 2 {
			continue
		}
		figures := map[int]bool{}
		holdings := map[string]bool{}
		for _, entry := range group {
			value, readable := numberFromWord(entry.figure)
			if !readable {
				t.Errorf("%s:%d: the figure %s cannot be read as a number, so nothing can hold it", entry.document, entry.line, entry.figure)
				continue
			}
			figures[value] = true
			holdings[entry.holding()] = true
		}
		if len(figures) < 2 && len(holdings) < 2 {
			continue
		}
		var members []string
		for _, entry := range group {
			members = append(members, fmt.Sprintf("    %s figure=%s %s", entry.where(), entry.figure, entry.holding()))
		}
		t.Errorf("the %d entries counting %q disagree about the figure or about how it is held:\n%s",
			len(group), phrase, strings.Join(members, "\n"))
	}
}

// TestEveryLedgerReferenceIsLive fails on any reference the ledger makes that
// the tree no longer answers: a noun matching nothing, a derivation named by
// no entry or naming nothing, a by= naming no declared test, and an at=
// pointing at a file or a line that does not carry what it says.
//
// The rule about a derivation nothing names is the second bound on the escape
// hatch. Where a set is stated only once in the corpus, the consistency rule
// has no sibling to compare against, so what stops a lone entry being
// converted away from its derivation is that the derivation is then registered
// and unread.
func TestEveryLedgerReferenceIsLive(t *testing.T) {
	nouns, entries := readProseFigures(t)
	documents := proseCorpus(t)
	for _, noun := range nouns {
		if nounStandsInTheCorpus(documents, noun) {
			continue
		}
		t.Errorf("%s registers the noun %q and it matches nothing in the corpus, so the scan carries a word no document uses", proseFigureLedger, noun)
	}
	named := map[string]bool{}
	for _, entry := range entries {
		if entry.derives == "" {
			continue
		}
		named[entry.derives] = true
		if _, known := derivationsByName[entry.derives]; !known {
			t.Errorf("%s:%d names the derivation %s and derivationsByName carries no such derivation", proseFigureLedger, entry.source, entry.derives)
		}
	}
	var registered []string
	for name := range derivationsByName {
		registered = append(registered, name)
	}
	sort.Strings(registered)
	for _, name := range registered {
		if named[name] {
			continue
		}
		t.Errorf("derivationsByName registers %s and no entry in %s names it, so a derivation stands in this file that nothing reads", name, proseFigureLedger)
	}
	declared := declaredTestFunctions(t)
	for _, entry := range entries {
		if entry.by != "" && !declared[entry.by] {
			t.Errorf("%s:%d rests the figure at %s on %s, and no test file in the module declares a function by that name",
				proseFigureLedger, entry.source, entry.where(), entry.by)
		}
		if entry.at == "" {
			continue
		}
		checkPointerIsLive(t, entry)
	}
}

// nounStandsInTheCorpus reports whether a registered noun is written anywhere
// in the corpus outside a fenced block. The test is the bare word rather than
// a figure beside it, because a noun the documents use without a count is
// still a noun this scan is right to watch.
func nounStandsInTheCorpus(documents []guardedDocument, noun string) bool {
	pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(noun) + `\b`)
	for _, document := range documents {
		for _, line := range proseLinesOf(document.text) {
			if line != "" && pattern.MatchString(line) {
				return true
			}
		}
	}
	return false
}

// checkPointerIsLive holds one holds=unreachable pointer to the tree: the file
// exists, and the line it names carries the identifier the entry declares.
//
// The identifier is required rather than the line merely being non-blank,
// because a non-blank test passes on a doc comment, on a closing brace, and on
// whatever the file grows next, which is not a check on a pointer at all.
func checkPointerIsLive(t *testing.T, entry proseEntry) {
	t.Helper()
	where, number, ok := strings.Cut(entry.at, ":")
	if !ok {
		t.Errorf("%s:%d writes at=%s, and a pointer names a file and a line", proseFigureLedger, entry.source, entry.at)
		return
	}
	at, err := strconv.Atoi(number)
	if err != nil || at < 1 {
		t.Errorf("%s:%d writes at=%s, whose line is not a number", proseFigureLedger, entry.source, entry.at)
		return
	}
	source, err := os.ReadFile(filepath.Join(repositoryRoot, filepath.FromSlash(where)))
	if err != nil {
		t.Errorf("%s:%d points at %s, which cannot be read: %v", proseFigureLedger, entry.source, entry.at, err)
		return
	}
	lines := strings.Split(strings.ReplaceAll(string(source), "\r\n", "\n"), "\n")
	if at > len(lines) {
		t.Errorf("%s:%d points at %s, and %s carries %d lines", proseFigureLedger, entry.source, entry.at, where, len(lines))
		return
	}
	word := regexp.MustCompile(`\b` + regexp.QuoteMeta(entry.declares) + `\b`)
	if !word.MatchString(lines[at-1]) {
		t.Errorf("%s:%d says %s declares %s, and line %d of %s does not carry that name:\n  %s",
			proseFigureLedger, entry.source, entry.at, entry.declares, at, where, strings.TrimSpace(lines[at-1]))
	}
}

// testFunctionDeclaration matches the declaration of a test or a test helper,
// which is what a by= pointer has to resolve to.
var testFunctionDeclaration = regexp.MustCompile(`(?m)^func ([A-Za-z0-9_]+)\(`)

// declaredTestFunctions names every function declared in a test file of the
// module, which is the set a by= pointer is resolved against.
func declaredTestFunctions(t *testing.T) map[string]bool {
	t.Helper()
	declared := map[string]bool{}
	err := filepath.Walk(repositoryRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if name := info.Name(); name == ".git" || name == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, found := range testFunctionDeclaration.FindAllStringSubmatch(string(source), -1) {
			declared[found[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the module's test files: %v", err)
	}
	if len(declared) == 0 {
		t.Fatal("no test file in the module declares a function, so a by= pointer would resolve against nothing")
	}
	return declared
}
