package main

import (
	"strings"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/verb"
)

// TestTheCommentHelpPageNamesSixPreconditionsInCheckOrder asserts dinah-502
// AC-8. dinah help comment prints six rows, in the order the verb checks
// them, being the harness row dinah-496 prepends followed by the five rows
// comment declares for itself, and the two rows dinah-502 adds take fresh
// catalog keys rather than the next ones in sequence: check.comment.4 and
// check.comment.5 rather than check.comment.5 renumbering nothing. The page
// carries no row for dinah.is-a-collection, following attach's own
// precedent, and the same run pins that the behaviour still fires though the
// page is silent about it.
//
// Arming: swapping two rows of beyondChecks["comment"] reddens the literal
// comparison and leaves the generated one green, on the same terms
// TestTheAttachHelpPageNamesTheKindPrecondition already arms itself.
func TestTheCommentHelpPageNamesSixPreconditionsInCheckOrder(t *testing.T) {
	root := newBench(t)
	t.Setenv("COLUMNS", "80")
	// The harness row heads this list at dinah-496, ahead of the five rows
	// comment declares for itself.
	wanted := []string{
		contract.MalformedHarness,
		contract.UnknownCard,
		contract.UnknownPath,
		contract.NoOwner,
		contract.NotCommentable,
		contract.Malformed,
	}
	wantedKeys := []string{
		"check.harness",
		"check.comment.1",
		"check.comment.4",
		"check.comment.2",
		"check.comment.5",
		"check.comment.3",
	}

	declared := verb.Checks("comment")
	if len(declared) != len(wanted) {
		t.Fatalf("comment declares %d preconditions, wanted %d: %+v", len(declared), len(wanted), declared)
	}
	for i, refusal := range wanted {
		if declared[i].Refusal != refusal {
			t.Errorf("precondition %d is %s, wanted %s", i+1, declared[i].Refusal, refusal)
		}
		if declared[i].Key != wantedKeys[i] {
			t.Errorf("precondition %d carries catalog key %s, wanted %s", i+1, declared[i].Key, wantedKeys[i])
		}
	}

	page := runCLI(t, root, "help", "comment")
	if page.code != 0 {
		t.Fatalf("help comment: %d %s", page.code, page.errw)
	}
	rows := refusalRowsOf(page.out)
	if len(rows) != len(wanted) {
		t.Fatalf("the page draws %d numbered rows, wanted %d:\n%s", len(rows), len(wanted), page.out)
	}
	for i, refusal := range wanted {
		if rows[i] != refusal {
			t.Errorf("row %d names %s, wanted %s", i+1, rows[i], refusal)
		}
	}
	if strings.Contains(page.out, string(contract.IsACollection)) {
		t.Errorf("help comment names %s, and this card follows attach in leaving it off the page:\n%s", contract.IsACollection, page.out)
	}
	if !strings.Contains(page.out, "dinah guide references") {
		t.Errorf("help comment does not point at dinah guide references:\n%s", page.out)
	}

	// The precedent: attach resolves through the same entity resolver and
	// its own page carries no row for the collection refusal either.
	attachPage := runCLI(t, root, "help", "attach")
	if attachPage.code != 0 {
		t.Fatalf("help attach: %d %s", attachPage.code, attachPage.errw)
	}
	if strings.Contains(attachPage.out, string(contract.IsACollection)) {
		t.Fatalf("help attach names %s, so this card's precedent does not hold:\n%s", contract.IsACollection, attachPage.out)
	}

	for _, line := range strings.Split(page.out, "\n") {
		if displayWidth(line) > 80 {
			t.Errorf("help comment draws a line %d columns wide:\n%q", displayWidth(line), line)
		}
	}
}

// TestCommentOnACollectionRefusesIsACollection asserts dinah-502 AC-8's
// second half: the refusal the page leaves undocumented still fires, raised
// by the reference grammar rather than by the verb, exactly as it does for
// every other roster command the collection sweep in
// collection_reference_test.go covers.
func TestCommentOnACollectionRefusesIsACollection(t *testing.T) {
	root := newBench(t)
	mustRun(t, root, "add", "a card with a comment")
	mustRun(t, root, "comment", "fx-1", "the first thought")

	got := runCLI(t, root, "comment", "fx-1/comments", "text", "--json")
	if got.code != 2 {
		t.Fatalf("comment fx-1/comments: exit %d, wanted 2:\n%s%s", got.code, got.out, got.errw)
	}
	if !strings.Contains(got.out, string(contract.IsACollection)) {
		t.Errorf("comment on a collection reference did not refuse %s:\n%s", contract.IsACollection, got.out)
	}
}
