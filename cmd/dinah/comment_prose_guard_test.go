package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// This file is the derivation behind dinah-518/criteria/16, and it exists
// because the transcription step is where that criterion lost a site. The
// specification's section 13 sweeps every source comment asserting a comment's
// containment, and section 12 lists the sites that sweep found. The list was
// retyped from the sweep's answer rather than computed from it, one site fell
// out between the two, and the falsified sentence shipped.
//
// So the population is walked here instead of being written down. A sweep that
// runs on every build cannot drop a row while somebody copies it.
//
// The clauses below were taken by running section 13's command at this
// branch's merge base, which `git merge-base origin/main HEAD` names, rather
// than at a revision somebody wrote into a document. Each answered at least
// one line there and answers none now, which is the arming the criterion asks
// for: a clause that never matched anything would pass this sweep for free.

// staleCommentContainment are the clauses that said a comment hangs below a
// card or below one of that card's checklist items, and nowhere else. Every
// one of them is false after dinah-518, which gave a column its own comments
// collection.
var staleCommentContainment = []string{
	"one comment below a card",
	"a comment entity under a card",
	"are the two kinds a comment hangs",
	"card or on one of",
}

// The claim about what the comment verb records cannot be guarded by a
// forbidden clause, and that is the second lesson this card paid for. The
// stale sentence and the corrected one open with the same words and diverge
// only in what follows, so a clause short enough to match the stale text also
// matches the corrected text, and one long enough to miss the corrected text
// misses two of the three stale sites as well. The rule is therefore written
// over the sentences the opening words select: a sentence may say what the
// verb records, provided it names the column somewhere in that same sentence.
//
// It asks whether the column is named anywhere in that sentence rather than
// whether the sentence continues with one particular spelling. An earlier
// draft of this guard tested a fixed continuation, which is a test for one
// author's word order wearing the doc comment of a test for the rule, and it
// reddened on honest sentences naming the column in another order while
// telling their author they had not named it.
//
// What the bound admits, said plainly rather than left for the next reader to
// discover. A sentence that denies a column its comments and then mentions
// columns for an unrelated reason before its full stop passes, because the
// test is containment of the word and not an account of what the sentence
// does with it. Refusing that sentence would take a reading of negation in
// English prose, and a guard that reads negation fires falsely on honest text
// in a way nobody here can adjudicate line by line, which costs more than the
// residual does. The residual is narrow: the word has to appear inside the
// one sentence that makes the claim, so the ordinary way a stale sentence
// goes stale, by saying a card and its items and stopping, is caught.
const (
	commentVerbClaim = "records a comment on a card"
	// commentVerbHolder is the word the sentence has to carry. Its lower
	// case is compared against the sentence's own lower case, so a
	// sentence opening on the word is admitted as readily as one carrying
	// it in the middle, and the plural spelling carries it too.
	commentVerbHolder = "column"
	// commentVerbSentenceCap bounds the search when a claim is followed by
	// neither a sentence terminator nor a line break, which happens when a
	// file's last comment runs to the end of the file. It is generous
	// enough that no sentence this repository writes reaches it.
	commentVerbSentenceCap = 320
)

// commentProseRoots are the three trees section 13's command reads.
//
// Each is derived from the repository root rather than spelled as a hop out of
// this package, because a hop is right only for the number of levels its
// author counted and a miscount reads as correct in review. findRepositoryRoot
// climbs to the directory holding go.mod and says which directory it found.
var commentProseRoots = []string{
	filepath.Join(repositoryRoot, "internal"),
	filepath.Join(repositoryRoot, "cmd"),
	filepath.Join(repositoryRoot, "editors", "vscode", "src"),
}

// commentProseContinuation matches the break between two lines of one comment,
// in both of the styles this repository writes: a `//` line in Go and a ` *`
// line inside a TypeScript block comment.
//
// Joining those lines before the scan is what stops the wrap trap, which has
// now cost this card two defects. A line-oriented grep answers nothing for a
// sentence that happens to wrap, so a clause quoted from a wrapped sentence
// tests nothing and reports that it tested something.
var commentProseContinuation = regexp.MustCompile(`[ \t]*\r?\n[ \t]*(?://|\*)[ \t]*`)

// What this guard does not see, so that nobody meeting it reads a green run as
// a statement about the repository. The list is open: these are the blind
// spots somebody has looked for and found, and a place not named below is
// unexamined rather than covered.
//
//   - The message catalogs under internal/msg/locales/, which carry the
//     published English a person actually reads. A stale sentence planted in
//     one of them leaves this guard green, because the walk admits .go and .ts
//     files and nothing else.
//   - The guides shipped inside the binary, under internal/guide/guides/, and
//     docs/ generally. The journal event table in docs/design/format.md went
//     stale on this very card underneath a green run of this test.
//   - A Go block comment whose continuation lines carry neither // nor *.
//     commentProseContinuation joins the two styles this repository writes and
//     no third, so a sentence wrapped in some other hanging style is scanned
//     as two fragments. No such comment stands under the three roots today,
//     which makes this one latent rather than live.
//   - A sentence wrapped across a concatenated Go string literal. The scan
//     reads whole file text and joins comment continuations alone, so a
//     sentence split over two adjacent literals is invisible to it, and the
//     three roots carry such wraps today.
//
// The first of those is demonstrated rather than asserted:
// TestTheCommentProseGuardSaysWhereItCannotSee plants a stale sentence in a
// catalog-shaped file and in a doc-shaped one and watches the walk decline
// both.

// TestNoProseSaysAColumnHasNoComments walks the population section 13 sweeps
// and fails on any surviving clause that denies a column its comments.
//
// The two file counts are kept apart for the reason the retired-spelling sweep
// keeps its own two apart: a single total stays comfortably non-zero when one
// walk's root has moved, and the half that reads nothing then passes in
// silence. The TypeScript half is the one that matters here, because the site
// this guard was written for is a TypeScript file.
func TestNoProseSaysAColumnHasNoComments(t *testing.T) {
	goFiles, tsFiles := 0, 0
	for _, root := range commentProseRoots {
		read := sweepCommentProse(t, root)
		goFiles += read.goFiles
		tsFiles += read.tsFiles
		for _, finding := range read.findings {
			t.Errorf("%s %s", finding.file, finding.message)
		}
	}
	if goFiles == 0 {
		t.Fatal("the Go half of the sweep read no file, so its roots have moved")
	}
	if tsFiles == 0 {
		t.Fatal("the TypeScript half of the sweep read no file, so its roots have moved")
	}
	t.Logf("the sweep read %d Go files and %d TypeScript files", goFiles, tsFiles)
}

// commentProseRead is how many files of each kind one walk admitted, and what
// it found in them.
type commentProseRead struct {
	goFiles  int
	tsFiles  int
	findings []commentProseFinding
}

// commentProseFinding is one stale statement, named by the file carrying it.
//
// The walk hands its findings back rather than reporting them itself, so that
// a check can run the walk over a tree of its own and read what it decided
// without failing the run. A guard whose decision is reachable only through a
// failed assertion cannot be attacked by a test, and this one has now been
// wrong twice about what it decides.
type commentProseFinding struct {
	file    string
	message string
}

// sweepCommentProse reads every non-test Go and TypeScript file below one
// root, joins each comment's continuation lines, and reports what it read.
//
// A test file is excluded because this file is one: the clauses it forbids are
// written out in it, and a sweep that read itself would fail on its own
// vocabulary. That exclusion is the one section 13 states and there is no
// other.
func sweepCommentProse(t *testing.T, root string) commentProseRead {
	t.Helper()
	var read commentProseRead
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		switch {
		case strings.HasSuffix(path, "_test.go"):
			return nil
		case strings.HasSuffix(path, ".test.ts"):
			return nil
		case strings.HasSuffix(path, ".go"):
			read.goFiles++
		case strings.HasSuffix(path, ".ts"):
			read.tsFiles++
		default:
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative := commentProseRelative(path)
		for _, message := range scanCommentProse(string(body)) {
			read.findings = append(read.findings, commentProseFinding{file: relative, message: message})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return read
}

// commentProseRelative spells a walked path relative to the repository root,
// which is how a finding names the file somebody has to open.
func commentProseRelative(path string) string {
	relative, err := filepath.Rel(repositoryRoot, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(relative)
}

// scanCommentProse decides what one file's text says, and is the whole of the
// guard's judgement. It takes text rather than a path so that a check can put
// a sentence through it directly and watch the answer.
func scanCommentProse(body string) []string {
	var messages []string
	joined := commentProseContinuation.ReplaceAllString(body, " ")
	for _, clause := range staleCommentContainment {
		if strings.Contains(joined, clause) {
			messages = append(messages, "carries "+strconv.Quote(clause)+", which says a comment hangs below a card and its items and nowhere else; a column holds comments too")
		}
	}
	for at := 0; ; {
		hit := strings.Index(joined[at:], commentVerbClaim)
		if hit < 0 {
			break
		}
		hit += at
		sentence := commentVerbSentenceAt(joined, hit)
		if !strings.Contains(strings.ToLower(sentence), commentVerbHolder) {
			messages = append(messages, "says what the comment verb records without naming the column in the sentence that says it: "+strconv.Quote(sentence))
		}
		at = hit + len(commentVerbClaim)
	}
	return messages
}

// commentVerbSentenceAt returns the sentence that begins at the claim, which
// is what bounds the search for the column.
//
// The bound is what makes the rule the rule its name states. Searching the
// whole file would admit a sentence denying a column its comments next door to
// an unrelated one mentioning columns; searching a fixed continuation admits
// one author's word order and no other. A sentence ends at the first of a
// terminator followed by white space, quotation or the end of the text, a line
// break the comment joiner did not swallow, or the cap.
func commentVerbSentenceAt(text string, at int) string {
	end := len(text)
	if at+commentVerbSentenceCap < end {
		end = at + commentVerbSentenceCap
	}
	for i := at; i < end; i++ {
		switch text[i] {
		case '\n', '\r':
			return text[at:i]
		case '.', '!', '?':
			if i+1 == len(text) {
				return text[at : i+1]
			}
			switch text[i+1] {
			case ' ', '\t', '\n', '\r', '"', '`', '\'':
				return text[at : i+1]
			}
		}
	}
	return text[at:end]
}

// TestTheColumnRuleAdmitsEveryOrderThatNamesTheColumn attacks the conditional
// rule with the sentences its earlier draft refused.
//
// That draft demanded one exact continuation, so a sentence naming the column
// second passed and a sentence naming it third or naming it in a clause of its
// own was reddened with a message saying the column had not been named. The
// rule the doc comment states is the one tested here: the column is named
// somewhere in the sentence that makes the claim, in whatever order the author
// found clearest. Each sentence below is wrapped across two comment lines as
// well, because the wrap is the shape that has already cost this card two
// defects.
func TestTheColumnRuleAdmitsEveryOrderThatNamesTheColumn(t *testing.T) {
	naming := []string{
		"records a comment on a card, on a column, or on one of that card's checklist items.",
		"records a comment on a card, on a checklist item, or on a column.",
		"records a comment on a card or on a column, and never on the workbench itself.",
	}
	for _, sentence := range naming {
		for _, message := range scanCommentProse(wrappedComment(sentence)) {
			if strings.Contains(message, "naming the column") {
				t.Errorf("the rule refuses %q, which names the column: %s", sentence, message)
			}
		}
	}

	silent := "records a comment on a card or on one of that card's checklist items, and on nothing else."
	refusals := 0
	for _, message := range scanCommentProse(wrappedComment(silent)) {
		if strings.Contains(message, "naming the column") {
			refusals++
		}
	}
	if refusals != 1 {
		t.Errorf("%d findings name the missing column for %q, wanted 1", refusals, silent)
	}
}

// wrappedComment writes a sentence as a Go doc comment broken across two
// lines, which is how the repository actually carries one.
func wrappedComment(sentence string) string {
	words := strings.Fields(sentence)
	half := len(words) / 2
	return "// The verb " + strings.Join(words[:half], " ") + "\n// " + strings.Join(words[half:], " ") + "\n"
}

// TestTheCommentProseGuardSaysWhereItCannotSee demonstrates the first blind
// spot the file header names, rather than leaving a reader to take the header
// on trust.
//
// One sentence is planted three times under one root, in a catalog-shaped
// file, in a document-shaped file and in a Go file. The Go file is the
// control: it proves the sentence is one this guard catches, so the silence
// over the other two is about the file's kind and not about the text.
func TestTheCommentProseGuardSaysWhereItCannotSee(t *testing.T) {
	root := t.TempDir()
	stale := "// The verb records a comment on a card or on one of\n// that card's checklist items, and on nothing else.\n"
	for _, name := range []string{"catalog.json", "guide.md", "seen.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(stale), 0o644); err != nil {
			t.Fatalf("plant %s: %v", name, err)
		}
	}

	read := sweepCommentProse(t, root)
	if read.goFiles != 1 {
		t.Fatalf("the walk read %d Go files under the planted root, wanted 1", read.goFiles)
	}
	if len(read.findings) == 0 {
		t.Fatal("the walk found nothing in the Go control, so the planted sentence is not one this guard catches and the silence below would prove nothing")
	}
	for _, finding := range read.findings {
		if !strings.HasSuffix(finding.file, "seen.go") {
			t.Errorf("a finding names %s, so the blind spots the file header claims are not blind: %s", finding.file, finding.message)
		}
	}
}
