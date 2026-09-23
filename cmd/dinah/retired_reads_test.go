package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// The guard over the seven read verbs dinah-523 retired.
//
// It follows cmd/dinah/retired_spellings_test.go in shape and differs from it
// in four respects the shape forces, each of which is written out where it
// lands: what the retired set holds, the normalisation that runs before any
// pattern does, the counted pair that guards the one verb whose creating form
// survives, and the help block, which is asserted rather than swept.

// retiredReadInvocations are the six invocations that no longer exist. The set
// is the invocation rather than the verb word, and that is the first
// difference from the sibling guard.
//
// The six verbs are single words English uses constantly. A whole-word sweep
// for log, contents, columns, attachments or ls would fire on hundreds of
// ordinary sentences and the allowlist would swallow the corpus, so what is
// matched is the two-word phrase a reader would type.
var retiredReadInvocations = []string{
	"dinah ls", "dinah columns", "dinah contents",
	"dinah attachments", "dinah log", "dinah workbenches",
}

// retiredReadPattern matches one retired invocation as two whole words, which
// is how the sibling guard matches its own.
var retiredReadPattern = func() []*regexp.Regexp {
	patterns := make([]*regexp.Regexp, 0, len(retiredReadInvocations))
	for _, invocation := range retiredReadInvocations {
		patterns = append(patterns, regexp.MustCompile(`\b`+regexp.QuoteMeta(invocation)+`\b`))
	}
	return patterns
}()

// The counted pair, which is the third difference. `dinah workstream` cannot
// join the list above because `dinah workstream new` survives and contains it,
// and Go's regular expressions carry no negative lookahead to write "not
// followed by new". An excuse cannot rescue it either, since an excuse is keyed
// to a file and a phrase and this would need one entry per file carrying a
// creating invocation, and a new entry every time a document gained one.
var (
	retiredWorkstreamAny = regexp.MustCompile(`\bdinah workstream\b`)
	retiredWorkstreamNew = regexp.MustCompile(`\bdinah workstream new\b`)
)

// retiredContinuation is the marker a wrapped line opens with in the corpora
// this sweep reads: a Go line comment, a Markdown heading, a bullet or a
// blockquote. It is stripped from the head of every line so that a two-word
// invocation broken across a line break reads as two words with one space
// between them.
//
// The hyphen is among the bullets and the asterisk is not, and the asymmetry is
// deliberate. This repository writes hyphen bullets and writes no asterisk
// ones, so stripping the asterisk would be closing the case the corpus never
// produces while leaving open the one it writes nineteen times.
var retiredContinuation = regexp.MustCompile(`(?m)^[ \t]*(?://+|#+|-|>)[ \t]*`)

// retiredWhitespace collapses a run of whitespace into one space, and leaves a
// run carrying two or more newlines alone. A paragraph break is not a line
// wrap, and joining across one would let a paragraph ending in the word dinah
// and the next beginning with a verb compose an invocation nobody wrote.
//
// The character class is spelled out rather than written \s because Go's \s is
// [\t\n\f\r ] and does not include a non-breaking space, which a document
// editor inserts without asking and which a reader cannot see.
var retiredWhitespace = regexp.MustCompile(`[\s\x{00a0}]+`)

// normaliseForRetirement rewrites one file's bytes so that a wrapped
// invocation reads as an invocation.
//
// Two shapes it cannot see, named here with the search that finds each, so
// that whoever meets one later arrives holding the reproduction rather than a
// surprise. Neither occurs in this corpus today.
//
// The first is a Go string concatenation putting the prefix on one line and the
// verb on the next:
//
//	log("dinah " +
//		"ls")
//
// A search for `dinah\s*"\s*\+` finds it, and it returns one hit today, at
// cmd/dinah/args.go, which composes an invocation at run time out of whatever
// the caller typed and names no verb at all. Closing the shape would mean
// parsing Go rather than reading it.
//
// The second is a translated string carrying the two-character escape for a
// newline between the two words:
//
//	"text": "run `dinah\nls` to see what this workbench carries"
//
// A search for `dinah\\+[nt]\s*\w+` finds it, and it returns nothing today.
// Closing it would mean reading every locale file twice under two grammars,
// once as JSON and once as text.
//
// Three further shapes were written against this normaliser and are not live in
// the corpus either. A Unicode space outside the declared class, such as an en
// space, a thin space, a narrow no-break space or a zero-width space, is not
// collapsed. A continuation marker outside the four stripped above, of which
// the asterisk bullet is the one a Markdown writer might reach for, is not
// stripped. And a Markdown backslash line continuation is not read as one.
func normaliseForRetirement(text string) string {
	text = retiredContinuation.ReplaceAllString(text, "")
	return retiredWhitespace.ReplaceAllStringFunc(text, func(run string) string {
		if strings.Count(run, "\n") >= 2 {
			return "\n\n"
		}
		return " "
	})
}

// retiredReadDocumentRoots are the document trees this sweep walks, which is
// the corpus the sibling guard walks, unchanged. It is not widened to
// editors/vscode/src, to docs/specs or to any live workbench: the extension
// builds argv arrays rather than writing prose, and is guarded by its own unit
// tests; the docs/specs sketches record what a card shipped at the time it
// shipped, and freezing them against a later retirement would make the record
// false; and the .dinah text is the operator's live data, which no test in this
// repository may assert against.
var retiredReadDocumentRoots = []string{
	filepath.Join("..", "..", "internal", "guide", "guides"),
	filepath.Join("..", "..", "internal", "msg", "locales"),
	filepath.Join("..", "..", "docs", "quick-start.md"),
	filepath.Join("..", "..", "docs", "design"),
}

// retiredReadGoRoots are the source trees this sweep walks, on the sibling
// guard's own reasoning for leaving test files out.
var retiredReadGoRoots = []string{
	filepath.Join("..", "..", "internal"),
	filepath.Join("..", "..", "cmd"),
}

// TestNoRetiredReadInvocationSurvivesInTheShippedCorpus normalises every file
// of the corpus and then finds no retired invocation in it, and no file where
// occurrences of `dinah workstream` outnumber occurrences of
// `dinah workstream new`.
//
// The two file counts are asserted separately, on the sibling guard's own
// reasoning: a single total is satisfied by the documents alone, so a Go walk
// whose root had moved would read nothing while the combined figure stayed
// comfortably non-zero and the run stayed green.
//
// This card declares no allowlist entry. Every occurrence it rewrote was prose
// teaching a reader or a maintainer to type a command that will not exist, and
// none of them was excusable.
func TestNoRetiredReadInvocationSurvivesInTheShippedCorpus(t *testing.T) {
	if len(retiredReadInvocations) == 0 {
		t.Fatal("no invocation is named as retired, so this sweep read nothing")
	}
	documents := 0
	for _, root := range retiredReadDocumentRoots {
		documents += sweepRetiredReads(t, root, func(string) bool { return true })
	}
	if documents == 0 {
		t.Fatal("the document walk read no file, so its roots have moved")
	}
	goFiles := 0
	for _, root := range retiredReadGoRoots {
		goFiles += sweepRetiredReads(t, root, func(path string) bool {
			return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
		})
	}
	if goFiles == 0 {
		t.Fatal("the Go walk read no file, so its roots have moved")
	}
	t.Logf("the sweep read %d documents and %d Go files", documents, goFiles)
}

// sweepRetiredReads walks one root, normalises every file the predicate
// admits, and reports how many it read.
func sweepRetiredReads(t *testing.T, root string, admits func(string) bool) int {
	t.Helper()
	read := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !admits(path) {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		read++
		relative := relativeToRepo(path)
		text := normaliseForRetirement(string(body))
		for at, invocation := range retiredReadInvocations {
			if retiredReadPattern[at].MatchString(text) {
				t.Errorf("%s carries the retired invocation %q", relative, invocation)
			}
		}
		if any, creating := retiredWorkstreamCounts(text); any > creating {
			t.Errorf("%s carries `dinah workstream` %d times and `dinah workstream new` %d times, so %d of them name the listing that retired",
				relative, any, creating, any-creating)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return read
}

// retiredWorkstreamCounts is the counted pair, over already normalised text: a
// wrapped bare invocation adds nothing to either side of the comparison and
// would defeat it in exactly the way a wrapped phrase defeats the patterns.
func retiredWorkstreamCounts(text string) (int, int) {
	return len(retiredWorkstreamAny.FindAllString(text, -1)),
		len(retiredWorkstreamNew.FindAllString(text, -1))
}

// helpSection is one section of the embedded help block: the heading it opens
// with, and the command of each usage row under it.
type helpSection struct {
	name     string
	commands []string
	usage    map[string]string
}

// helpSections cuts the embedded block into its sections and reads the usage
// rows out of each.
//
// A section opens with a line matching ^[A-Z]+$ and runs to the next blank
// line. Inside one, a usage row is a line matching `^  [^ ]`, and the command
// is its first whitespace-separated token. That rule separates a usage row from
// the two things it could be confused with, because a continuation row of flags
// is indented four spaces and a wrapped description line is indented to the
// description column, and neither matches. It also stops the scan reaching the
// global-flags table at the foot of the block, whose rows sit outside every
// section.
func helpSections(block string) []helpSection {
	heading := regexp.MustCompile(`^[A-Z]+$`)
	row := regexp.MustCompile(`^  [^ ]`)
	var sections []helpSection
	var current *helpSection
	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimRight(line, " ")
		if heading.MatchString(trimmed) {
			sections = append(sections, helpSection{name: trimmed, usage: map[string]string{}})
			current = &sections[len(sections)-1]
			continue
		}
		if current == nil {
			continue
		}
		if strings.TrimSpace(trimmed) == "" {
			current = nil
			continue
		}
		if !row.MatchString(trimmed) {
			continue
		}
		fields := strings.Fields(trimmed)
		current.commands = append(current.commands, fields[0])
		current.usage[fields[0]] = strings.TrimSpace(trimmed)
	}
	return sections
}

// TestTheHelpBlockCarriesTheCollapsedReadSurface asserts what the block shows
// rather than sweeping it, which is the fourth difference from the sibling
// guard: this card's phrases carry a `dinah ` prefix the block never prints, so
// a sweep over the block would compare nothing.
func TestTheHelpBlockCarriesTheCollapsedReadSurface(t *testing.T) {
	sections := helpSections(tableSession(80).helpBlock())
	if len(sections) == 0 {
		t.Fatal("the block carries no section, so every assertion below would pass on an empty read")
	}
	rows, seen := 0, map[string]string{}
	var read, workbench *helpSection
	for at := range sections {
		section := &sections[at]
		rows += len(section.commands)
		for _, command := range section.commands {
			if where, twice := seen[command]; twice {
				t.Errorf("the block lists %s under %s and under %s", command, where, section.name)
			}
			seen[command] = section.name
		}
		switch section.name {
		case "READ":
			read = section
		case "WORKBENCH":
			workbench = section
		}
	}
	if read == nil || workbench == nil {
		t.Fatal("the block carries no READ section or no WORKBENCH section")
	}

	// The six verbs that leave the command table, asserted over every section
	// rather than over the READ section alone, because a retired verb
	// reappearing under another heading is the shape this would otherwise miss.
	for _, retired := range []string{"ls", "columns", "contents", "attachments", "log", "workbenches"} {
		if where, listed := seen[retired]; listed {
			t.Errorf("the block still lists %s, under %s", retired, where)
		}
	}
	wantedRead := []string{"status", "list", "next", "query", "search", "tree", "show", "changes", "instructions", "prime", "guide"}
	if strings.Join(read.commands, " ") != strings.Join(wantedRead, " ") {
		t.Errorf("the READ section lists [%s], and it lists [%s]", strings.Join(read.commands, " "), strings.Join(wantedRead, " "))
	}

	// The seventh verb is accounted for here rather than among the six above,
	// because its creating action survives and its row stays.
	workstreams := 0
	for _, command := range workbench.commands {
		if command == "workstream" {
			workstreams++
		}
	}
	if workstreams != 1 {
		t.Errorf("the WORKBENCH section carries %d rows beginning workstream, wanted one", workstreams)
	}
	if want := "workstream <new> <title> [--slug <slug>]"; !strings.HasPrefix(workbench.usage["workstream"], want) {
		t.Errorf("the workstream row reads %q, and its usage is %q", workbench.usage["workstream"], want)
	}
	if rows != 58 {
		t.Errorf("the four sections carry %d usage rows, wanted fifty-eight", rows)
	}
}

// TestTheSevenRetiredReadsRefuseAndTheirReplacementsWork asserts, per verb, the
// refusal a reader now meets and the invocation that replaces it.
//
// The replacement half is what stops this passing against a tree where the
// reads do not work at all: a build that refused everything would fail the
// second column of every row.
func TestTheSevenRetiredReadsRefuseAndTheirReplacementsWork(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "a card to read about"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "workstream", "new", "Autumn release", "--slug", "autumn"); got.code != 0 {
		t.Fatalf("workstream new: %d %s", got.code, got.errw)
	}
	rows := []struct {
		retired     []string
		wanted      string
		replacement []string
		prints      string
	}{
		{
			retired:     []string{"ls", "intake"},
			wanted:      contract.UnknownVerb,
			replacement: []string{"list", "intake"},
			prints:      "a card to read about",
		},
		{
			retired:     []string{"columns"},
			wanted:      contract.UnknownVerb,
			replacement: []string{"list", "columns"},
			prints:      "intake",
		},
		{
			retired:     []string{"contents", "fx-1"},
			wanted:      contract.UnknownVerb,
			replacement: []string{"list", "fx-1"},
			prints:      "fx-1",
		},
		{
			retired:     []string{"attachments", "fx-1"},
			wanted:      contract.UnknownVerb,
			replacement: []string{"list", "fx-1/attachments"},
			prints:      "fx-1",
		},
		{
			retired:     []string{"log", "fx-1"},
			wanted:      contract.UnknownVerb,
			replacement: []string{"list", "fx-1/journal"},
			prints:      "created",
		},
		{
			retired:     []string{"workbenches"},
			wanted:      contract.UnknownVerb,
			replacement: []string{"list", "workbenches"},
			prints:      "fx",
		},
		{
			// The seventh verb keeps its command and loses its bare action, so
			// its unknown first word falls through to the usage refusal, which
			// is the shape the sibling guard's `workbench get` row already
			// uses.
			retired:     []string{"workstream"},
			wanted:      contract.Usage,
			replacement: []string{"list", "workstreams"},
			prints:      "Autumn release",
		},
	}
	if len(rows) != len(retiredReadInvocations)+1 {
		t.Fatalf("this check runs %d rows, and the retirement is %d invocations plus the counted-pair verb", len(rows), len(retiredReadInvocations))
	}
	for _, row := range rows {
		invocation := strings.Join(row.retired, " ")
		refused := runCLI(t, root, row.retired...)
		if refused.code != contract.ExitCode(contract.OutcomeRefused) {
			t.Errorf("`dinah %s` exited %d, wanted the refused exit code", invocation, refused.code)
		}
		if name := refusalNameOf(refused.errw); name != row.wanted {
			t.Errorf("`dinah %s` refused %s, wanted %s", invocation, name, row.wanted)
		}
		replacement := strings.Join(row.replacement, " ")
		answered := runCLI(t, root, row.replacement...)
		if answered.code != 0 {
			t.Errorf("`dinah %s` exited %d: %s", replacement, answered.code, answered.errw)
			continue
		}
		if !strings.Contains(answered.out, row.prints) {
			t.Errorf("`dinah %s` printed %q, wanted it to carry %q", replacement, answered.out, row.prints)
		}
	}
	if got := runCLI(t, root, "workstream", "new", "Winter release", "--slug", "winter"); got.code != 0 {
		t.Errorf("the creating action of the surviving command refused: %d %s", got.code, got.errw)
	}
}
