package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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
// over the lines the opening words select: a sentence may say what the verb
// records, provided it names the column among the kinds.
const (
	commentVerbClaim  = "records a comment on a card"
	commentVerbColumn = "records a comment on a card, on a column"
)

// commentProseRoots are the three trees section 13's command reads.
var commentProseRoots = []string{
	filepath.Join("..", "..", "internal"),
	filepath.Join("..", "..", "cmd"),
	filepath.Join("..", "..", "editors", "vscode", "src"),
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
	}
	if goFiles == 0 {
		t.Fatal("the Go half of the sweep read no file, so its roots have moved")
	}
	if tsFiles == 0 {
		t.Fatal("the TypeScript half of the sweep read no file, so its roots have moved")
	}
	t.Logf("the sweep read %d Go files and %d TypeScript files", goFiles, tsFiles)
}

// commentProseRead is how many files of each kind one walk admitted.
type commentProseRead struct {
	goFiles int
	tsFiles int
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
		relative := relativeToRepo(path)
		joined := commentProseContinuation.ReplaceAllString(string(body), " ")
		for _, clause := range staleCommentContainment {
			if strings.Contains(joined, clause) {
				t.Errorf("%s carries %q, which says a comment hangs below a card and its items and nowhere else; a column holds comments too", relative, clause)
			}
		}
		for at := 0; ; {
			hit := strings.Index(joined[at:], commentVerbClaim)
			if hit < 0 {
				break
			}
			hit += at
			if !strings.HasPrefix(joined[hit:], commentVerbColumn) {
				t.Errorf("%s says what the comment verb records without naming the column: %q", relative, excerptAround(joined, hit))
			}
			at = hit + len(commentVerbClaim)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return read
}

// excerptAround quotes enough of the offending sentence for a reader to find
// it without opening the file, and no more.
func excerptAround(text string, at int) string {
	end := at + 96
	if end > len(text) {
		end = len(text)
	}
	return text[at:end]
}
