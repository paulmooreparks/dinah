package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
)

// TestTheChecklistRowDrawsTheAnswerAnItemDesignates asserts the terminal half
// of dinah-525/criteria/19 and the rendering the spec's own "Rendering, and
// what the at-a-glance line costs" section fixes: the indexed checklist read
// carries the designated comment's reference in a column of its own, the full
// read draws the comment under the row it belongs to, and a comment the store
// cannot attribute is drawn as one the store cannot name.
//
// The unattributable case is the one worth having here rather than only in the
// library. The library asserts that the absence travels; this asserts that the
// sentence a person reads says so, which is the difference between a finding
// recorded and a finding reported.
func TestTheChecklistRowDrawsTheAnswerAnItemDesignates(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "file", "fx-1", "open_question", "Does the deadline move?"); got.code != 0 {
		t.Fatalf("file: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "file", "fx-1", "open_question", "Does the price move?"); got.code != 0 {
		t.Fatalf("file: %d %s", got.code, got.errw)
	}

	// The first question is settled by somebody the store can name.
	if got := runCLI(t, root, "resolve", "fx-1/questions/1", "--text", "the operator ruled it does not"); got.code != 0 {
		t.Fatalf("resolve: %d %s", got.code, got.errw)
	}
	// The second designates a comment written the way the note migration
	// writes one for an item whose settling the journal cannot attribute: no
	// author at all, and the absence recorded as a finding.
	plantAuthorlessComment(t, root, "fx-1/questions/2", "the answer somebody wrote")
	if got := runCLI(t, root, "resolve", "fx-1/questions/2", "fx-1/questions/2/comments/1"); got.code != 0 {
		t.Fatalf("resolve against the authorless comment: %d %s", got.code, got.errw)
	}

	// The indexed read carries the references and opens nothing.
	indexed := runCLI(t, root, "show", "fx-1")
	if indexed.code != 0 {
		t.Fatalf("show: %d %s", indexed.code, indexed.errw)
	}
	for _, ref := range []string{
		"fx-1/questions/1/comments/1",
		"fx-1/questions/2/comments/1",
	} {
		if !strings.Contains(indexed.out, ref) {
			t.Errorf("the indexed checklist does not name the answer %s:\n%s", ref, indexed.out)
		}
	}
	if strings.Contains(indexed.out, "the operator ruled it does not") {
		t.Errorf("the indexed checklist drew a comment's body, which costs a file open per item:\n%s", indexed.out)
	}

	// The full read draws both comments, and says of the second that the
	// store cannot name its author rather than leaving a blank or inventing
	// one.
	whole := runCLI(t, root, "show", "fx-1", "--fields", "checklist.full")
	if whole.code != 0 {
		t.Fatalf("show in full: %d %s", whole.code, whole.errw)
	}
	if !strings.Contains(whole.out, "the operator ruled it does not") {
		t.Errorf("the full checklist drew no answer for the attributed item:\n%s", whole.out)
	}
	if !strings.Contains(whole.out, "an author this workbench cannot name") {
		t.Errorf("the full checklist did not say the store cannot name the author:\n%s", whole.out)
	}
	if !strings.Contains(whole.out, "the answer somebody wrote") {
		t.Errorf("the full checklist drew no body for the unattributable answer:\n%s", whole.out)
	}
	if strings.Contains(whole.out, "unknown") {
		t.Errorf("the full checklist filled the absent author with a word:\n%s", whole.out)
	}

	// And check reports nothing about it: an absent author is a recorded
	// finding rather than a defect in the store.
	report := runCLI(t, root, "check")
	if report.code != 0 {
		t.Errorf("a store carrying an authorless comment does not check clean: %d\n%s", report.code, report.out)
	}
}

// plantAuthorlessComment writes one comment below an item with no author at
// all and author_unrecoverable on its anchor, which is what the note migration
// writes for a settling the journal cannot attribute.
//
// It is planted rather than made by a verb because no verb writes one: every
// comment a verb writes carries the actor that wrote it, and the state under
// test is the one the migration leaves behind.
func plantAuthorlessComment(t *testing.T, root, item, body string) {
	t.Helper()
	answer := runCLI(t, root, "path", item)
	if answer.code != 0 {
		t.Fatalf("path %s: %d %s", item, answer.code, answer.errw)
	}
	dir := filepath.Join(filepath.Dir(strings.TrimSpace(answer.out)), bench.CommentsDir, "f00000000001")
	fm := bench.NewFrontmatter()
	fm.Set("ts", "2026-08-01T09:00:00Z")
	fm.Set(bench.CommentAuthorUnrecoverableField, "true")
	fm.Set(bench.OrdinalField, "1")
	if err := bench.WriteCommentAnchor(dir, fm, body); err != nil {
		t.Fatalf("plant the authorless comment: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, bench.CommentAnchor)); err != nil {
		t.Fatalf("the planted comment is not on disk: %v", err)
	}
}
