package profile

import (
	"regexp"
	"strings"
	"testing"
)

// The facts this file holds still, so that an edit cannot move one of them
// quietly. The first three are what dinah-201's boundary-row amendment is not
// allowed to move. D-8 rules that an amendment changing no extracted statement
// takes no version increment and no changelog entry, and AC-9 says so in as
// many words, so all three are recorded here rather than left to a reviewer's
// eye. The fourth is dinah-203's, and it belongs beside them for the same
// reason.
//
// A later card that legitimately changes a statement changes these numbers as
// part of its own work, and having to edit this file is the point: DOC-CHG-2
// asks a changelog entry for the identifiers an increment affects, and the
// edit is where somebody is made to ask whether one is owed. The document
// states its own revision in five present-tense places, and the two constants
// below hold two of those five to their wording. Whether all five carry the
// same value is the separate question dinah-321 answers, in the test at the
// foot of this file.
const (
	// declaredVersion is the version sentence of section 2, in full, so a
	// bump cannot slip through as a one-character diff.
	declaredVersion = "This document is version 0.11 of the profile whose identity string is\n`dinah-core`."
	// publishedStatements is how many statements the document publishes.
	publishedStatements = 136
	// publishedChangelogEntries is how many entries section 12 carries. The
	// changelog is append-only under DOC-CHG-1, so this number never falls.
	publishedChangelogEntries = 11
	// declaredCurrentRevision is the sentence dinah-203 put at the head of
	// section 12, where a reader scanning the changelog meets 1.0, 2.0 and 3.0
	// before meeting anything that says which revision is live. It states the
	// revision a second time in this document, and an unread second copy is
	// how the header at line 3 went stale in the first place, so it is read
	// here.
	declaredCurrentRevision = "The current revision is `dinah-core 0.11`."
)

// changelogEntry matches an entry heading of section 12, which carries the
// version, the channel and the date.
var changelogEntry = regexp.MustCompile("(?m)^### [0-9]+\\.[0-9]+, channel `[a-z]+`, [0-9]{4}-[0-9]{2}-[0-9]{2}$")

// documentHeaderRevision matches the version-identity line at the head of the
// document, which is line 3 today and which no test read at all before
// dinah-321.
var documentHeaderRevision = regexp.MustCompile(
	"(?m)^Version identity: `dinah-core ([0-9]+\\.[0-9]+)`, maturity channel `[a-z]+`\\.$")

// documentSection2Revision matches section 2's version sentence, which
// dinah-321 D-1 makes the declaration every other site is checked against.
var documentSection2Revision = regexp.MustCompile(
	"This document is version ([0-9]+\\.[0-9]+) of the profile whose identity string is\n`dinah-core`\\.")

// documentSection21Revision matches section 2.1's example of what a conformance
// claim names. The sentence illustrates what CORE-VER-1 and CORE-VER-2 require
// of a claim, and the number in it states this document's own current revision,
// so an increment that passes it by leaves it naming a revision the document
// has left behind.
var documentSection21Revision = regexp.MustCompile(
	"A conformance claim names `dinah-core ([0-9]+\\.[0-9]+)` and says nothing about\nthe channel,")

// documentSection57Revision matches the profile member of the interchange-form
// example in section 5.7, which spells the revision after a slash rather than
// after a space.
var documentSection57Revision = regexp.MustCompile(
	`"profile": "dinah-core/([0-9]+\.[0-9]+)",`)

// documentSection12Revision matches the sentence at the head of section 12.
// declaredCurrentRevision above holds that sentence to its wording; this
// pattern captures its number, so the sentence joins the agreement check too.
var documentSection12Revision = regexp.MustCompile(
	"The current revision is `dinah-core ([0-9]+\\.[0-9]+)`\\.")

// TestAnEditorialAmendmentMovesNeitherVersionNorChangelog is dinah-201 AC-9,
// the half that says the amendment is editorial. The statement-list assertion
// is what makes the other two meaningful: an edit that changes no statement
// affects no identifier, so it owes no entry and moves no number.
//
// dinah-203 added the fourth assertion, over the sentence at the head of
// section 12. That sentence names the live revision, and until this test read
// it nothing did, which is the same condition that let the header at line 3
// sit two revisions behind.
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
	if !strings.Contains(text, declaredCurrentRevision) {
		t.Errorf("the head of section 12 no longer reads: %s", declaredCurrentRevision)
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

// TestAllOfTheProfilesRevisionStatementsAgree is dinah-321. The document states
// its own current revision in five present-tense places: the header, section 2,
// section 2.1's example of a conformance claim, section 5.7's interchange-form
// example, and the sentence at the head of section 12. declaredVersion and
// declaredCurrentRevision above hold two of the five to their wording, and
// until this test nothing held any of the five to the value the others carry,
// which is the condition that let the header sit two revisions behind section
// 2.
//
// Every value compared here is read out of the document as the test runs
// rather than recorded in this file, so an increment that moves all five
// together leaves this test alone, and an increment that moves fewer than five
// is the drift the test exists to catch. Section 2 is the reference the other
// four are checked against, per D-1.
//
// Two further occurrences of the revision number are deliberately not read.
// Section 3.5 records that a word left the closed vocabulary at 0.7, and
// section 12 carries an entry heading for the 0.7 entry itself. Both name a
// past event rather than the current revision, both stay at 0.7 after the next
// increment, and reading either here would redden this test on the first
// legitimate increment.
//
// internal/bench's ProfileMajor and ProfileMinor are not read here either, and
// the operator ruled on that in D-3. Those constants say which profile
// revisions this build conforms to, which is the ceiling of the window
// bench.admitProfile applies rather than a second copy of what the document
// currently says. Section 2 puts it in the document's own words, that the
// version of the profile is unrelated to the release numbering of any tool. A
// build may conform to an older revision than the one published, so a guard
// requiring the two to agree would redden a build for doing what the profile
// permits.
func TestAllOfTheProfilesRevisionStatementsAgree(t *testing.T) {
	text := readProfile(t)

	section2 := documentSection2Revision.FindStringSubmatch(text)
	if section2 == nil {
		t.Fatalf("section 2 no longer reads %q with a version in place of the number, so documentSection2Revision needs updating to match its new wording", declaredVersion)
	}
	reference := section2[1]

	sites := []struct {
		name string
		re   *regexp.Regexp
	}{
		{"the header", documentHeaderRevision},
		{"section 2.1's conformance-claim example", documentSection21Revision},
		{"section 5.7's interchange-form example", documentSection57Revision},
		{"the head of section 12", documentSection12Revision},
	}

	for _, site := range sites {
		m := site.re.FindStringSubmatch(text)
		if m == nil {
			t.Fatalf("%s no longer matches the wording this test reads it by, so its revision cannot be compared; update its pattern to the new wording", site.name)
		}
		if m[1] != reference {
			t.Errorf("section 2 declares revision %s and %s declares revision %s; an increment moved one and left the other behind", reference, site.name, m[1])
		}
	}
}
