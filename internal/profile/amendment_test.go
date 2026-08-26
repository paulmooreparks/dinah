package profile

import (
	"regexp"
	"strings"
	"testing"
)

// The three facts dinah-201's boundary-row amendment is not allowed to move.
// D-8 rules that an amendment changing no extracted statement takes no version
// increment and no changelog entry, and AC-9 says so in as many words, so all
// three are recorded here rather than left to a reviewer's eye.
//
// A later card that legitimately changes a statement changes these numbers as
// part of its own work, and having to edit this file is the point: DOC-CHG-2
// asks a changelog entry for the identifiers an increment affects, and the
// edit is where somebody is made to ask whether one is owed.
const (
	// declaredVersion is the version sentence of section 2, in full, so a
	// bump cannot slip through as a one-character diff.
	declaredVersion = "This document is version 0.6 of the profile whose identity string is\n`dinah-core`."
	// publishedStatements is how many statements the document publishes.
	publishedStatements = 129
	// publishedChangelogEntries is how many entries section 12 carries. The
	// changelog is append-only under DOC-CHG-1, so this number never falls.
	publishedChangelogEntries = 6
)

// changelogEntry matches an entry heading of section 12, which carries the
// version, the channel and the date.
var changelogEntry = regexp.MustCompile("(?m)^### [0-9]+\\.[0-9]+, channel `[a-z]+`, [0-9]{4}-[0-9]{2}-[0-9]{2}$")

// TestAnEditorialAmendmentMovesNeitherVersionNorChangelog is dinah-201 AC-9,
// the half that says the amendment is editorial. The statement-list assertion
// is what makes the other two meaningful: an edit that changes no statement
// affects no identifier, so it owes no entry and moves no number.
func TestAnEditorialAmendmentMovesNeitherVersionNorChangelog(t *testing.T) {
	text := readProfile(t)
	doc := extractProfile(t)

	if len(doc.Statements) != publishedStatements {
		t.Errorf("the profile publishes %d statements, and this file records %d; a card that changed the list owes DOC-CHG-2 an entry and this constant an edit",
			len(doc.Statements), publishedStatements)
	}
	if !strings.Contains(text, declaredVersion) {
		t.Errorf("section 2 no longer reads:\n%s", declaredVersion)
	}
	if entries := changelogEntry.FindAllString(text, -1); len(entries) != publishedChangelogEntries {
		t.Errorf("section 12 carries %d entries, and this file records %d: %v",
			len(entries), publishedChangelogEntries, entries)
	}
}

// TestTheBoundaryRowForWaitingOnSomebodyOutsideStands is the other half of
// dinah-201 AC-9. The row keeps its `out` ruling, and its reason now carries
// the two things the review asked for: that the earlier reason's second clause
// survives, and why a reopen condition this card arguably satisfies is being
// replaced rather than acted on.
func TestTheBoundaryRowForWaitingOnSomebodyOutsideStands(t *testing.T) {
	rows := BoundaryTable(readProfile(t))
	var row *BoundaryRow
	for i := range rows {
		if rows[i].Item == "A column where the workbench waits on somebody outside it" {
			row = &rows[i]
		}
	}
	if row == nil {
		t.Fatal("the boundary table no longer carries the row this card amends")
	}
	if row.Ruling != "out" {
		t.Errorf("the row is ruled %q, and dinah-201 D-2 leaves it out", row.Ruling)
	}
	for _, wanted := range []string{
		"a distinct kind would still add machinery for the same result",
		"nothing joins the core vocabulary that did not work somewhere first",
		"as a property of a column rather than a kind of column",
	} {
		if !strings.Contains(row.Reason, wanted) {
			t.Errorf("the row's reason does not carry %q", wanted)
		}
	}
	if strings.Contains(row.Reopen, "the block verb cannot express") {
		t.Error("the row still carries the reopen condition this card fired, which the amendment replaces")
	}
	if !strings.Contains(row.Reopen, "a second implementation needs to read it") {
		t.Errorf("the row's reopen condition is not the replacement: %q", row.Reopen)
	}
}
