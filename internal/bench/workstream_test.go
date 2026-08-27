package bench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestABareWorkstreamReferenceTakesTheIdentifierBeforeTheSlug asserts the
// resolution order a bare reference follows, and the case that makes the order
// load-bearing rather than tidy: the slug grammar admits twelve characters
// drawn from the letters a to f, so one workstream's slug can be another
// workstream's identifier.
func TestABareWorkstreamReferenceTakesTheIdentifierBeforeTheSlug(t *testing.T) {
	root := newFixture(t)
	writeWorkstream(t, root, "abcdefabcdef", "title: The hex one\nslug: hexone\nstatus: active\nordinal: 1\n")
	writeWorkstream(t, root, "f00000000002", "title: The slugged one\nslug: abcdefabcdef\nstatus: active\nordinal: 2\n")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	found := opened.WorkstreamByRef("abcdefabcdef")
	if found == nil {
		t.Fatal("the reference resolved to nothing")
	}
	if found.ID != "abcdefabcdef" {
		t.Errorf("the reference resolved to %s, and the identifier wins over another workstream's slug", found.ID)
	}
	if opened.WorkstreamByRef("hexone").ID != "abcdefabcdef" {
		t.Error("a slug no identifier shadows did not resolve")
	}
	if got := opened.WorkstreamByRef("The hex one"); got != nil {
		t.Errorf("a title resolved to workstream %s, and no surface names a workstream by its title", got.ID)
	}
}

// TestAWorkstreamResolvesInEitherHalfOfTheCollection asserts that an archived
// workstream still answers to its reference, which is what keeps a card
// belonging to a finished effort from becoming a dangler.
func TestAWorkstreamResolvesInEitherHalfOfTheCollection(t *testing.T) {
	root := newFixture(t)
	write(t, filepath.Join(root, ArchiveDir, WorkstreamsDir, "f00000000003", WorkstreamAnchor),
		"---\ntitle: A finished effort\nslug: finished-effort\nstatus: finished\nordinal: 1\n---\n")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !opened.HasWorkstream("f00000000003") {
		t.Error("an archived workstream does not resolve, so every card listing it became a dangler")
	}
	if opened.WorkstreamByRef("finished-effort") == nil {
		t.Error("an archived workstream does not answer to its slug")
	}
	if got := opened.Workstreams(); len(got) != 0 {
		t.Errorf("the live listing carries %d workstreams, wanted none", len(got))
	}
}

// TestWritingAWorkstreamFieldKeepsEveryKeyItDoesNotKnow asserts what the tool
// promises a file it did not write: a key Dinah does not recognise survives a
// write, in its own place in the header, and so does the body.
func TestWritingAWorkstreamFieldKeepsEveryKeyItDoesNotKnow(t *testing.T) {
	root := newFixture(t)
	writeWorkstream(t, root, "f00000000001",
		"title: Portfolio work\nslug: portfolio\nowner: somebody\nstatus: active\nordinal: 1\n")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	workstream := opened.WorkstreamByRef("portfolio")
	if workstream == nil {
		t.Fatal("the fixture workstream does not resolve")
	}
	workstream.SetField("status", "paused")
	if err := workstream.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	text, err := ReadText(workstream.AnchorPath())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := "---\ntitle: Portfolio work\nslug: portfolio\nowner: somebody\nstatus: paused\nordinal: 1\n---\nNotes.\n"
	if text != want {
		t.Errorf("the anchor reads\n%q\nwanted\n%q", text, want)
	}
}

// TestAnAnchorWithNoParseableHeaderLoadsWithEmptyFields asserts the third of
// the three hand-written cases: a workstream.md carrying prose rather than
// frontmatter loads rather than refusing, and the slug findings are what name
// it.
func TestAnAnchorWithNoParseableHeaderLoadsWithEmptyFields(t *testing.T) {
	root := newFixture(t)
	write(t, filepath.Join(root, WorkstreamsDir, "f00000000001", WorkstreamAnchor), "just a line of prose\n")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	workstreams := opened.Workstreams()
	if len(workstreams) != 1 {
		t.Fatalf("the listing carries %d workstreams, wanted the one on disk", len(workstreams))
	}
	if workstreams[0].Title != "" || workstreams[0].Slug != "" {
		t.Errorf("the workstream loaded as %+v, wanted empty fields", workstreams[0])
	}
	if workstreams[0].Ref() != "f00000000001" {
		t.Errorf("its reference is %q, and a workstream carrying no slug falls back to its identifier", workstreams[0].Ref())
	}
	findings, err := opened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	named := false
	for _, finding := range findings {
		if finding.Key == FindingWorkstreamSlugMissing {
			named = true
		}
	}
	if !named {
		t.Errorf("check reported %+v, wanted the slug finding that names an unnamed workstream", findings)
	}
}

// TestTheAdoptionRepairKeepsTheIdentifierAndWritesNoCard asserts what
// --migrate-workstreams does about a membership written before the tool could
// read a workstream: it creates the entity at the identifier the cards already
// name, so no card file is touched and every reference already written down
// still resolves.
func TestTheAdoptionRepairKeepsTheIdentifierAndWritesNoCard(t *testing.T) {
	root := newFixture(t)
	edit(t, root, "state: ready", "state: ready\nworkstreams:\n  - f00000000009")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	card := filepath.Join(root, CardsDir, "c00000000001", CardAnchor)
	before, err := ReadText(card)
	if err != nil {
		t.Fatalf("read the card: %v", err)
	}
	dangling, err := opened.DanglingWorkstreams()
	if err != nil {
		t.Fatalf("read the danglers: %v", err)
	}
	if strings.Join(dangling, ",") != "f00000000009" {
		t.Fatalf("the danglers read %v, wanted the one identifier no workstream answers to", dangling)
	}
	adopted, err := opened.AdoptWorkstream(dangling[0])
	if err != nil {
		t.Fatalf("adopt: %v", err)
	}
	if adopted.ID != "f00000000009" || adopted.Title != "" || adopted.Slug != "" || adopted.Status != StatusActive {
		t.Errorf("the adopted workstream reads %+v, wanted the identifier kept, no title, no slug and an active status", adopted)
	}
	after, err := ReadText(card)
	if err != nil {
		t.Fatalf("read the card again: %v", err)
	}
	if after != before {
		t.Errorf("the repair rewrote a card anchor:\n%q\n%q", before, after)
	}
	reopened, err := Open(root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if !reopened.HasWorkstream("f00000000009") {
		t.Error("the adopted identifier still resolves to nothing")
	}
	assigned, reported := reopened.BackfillWorkstreamSlugs()
	if len(assigned) != 0 {
		t.Errorf("the slug migration assigned %+v to a workstream carrying no title", assigned)
	}
	underivable := false
	for _, finding := range reported {
		if finding.Key == FindingWorkstreamSlugUnderivable {
			underivable = true
		}
	}
	if !underivable {
		t.Errorf("the slug migration reported %+v, wanted the finding that names a title no slug can be derived from", reported)
	}
}

// TestTheSlugMigrationRepairsAWorkstreamOnceItHasATitle asserts the second half
// of the repair the adoption leaves behind, including the collision suffix.
func TestTheSlugMigrationRepairsAWorkstreamOnceItHasATitle(t *testing.T) {
	root := newFixture(t)
	writeWorkstream(t, root, "f00000000001", "title: Portfolio work\nstatus: active\nordinal: 1\n")
	writeWorkstream(t, root, "f00000000002", "title: Portfolio work\nstatus: active\nordinal: 2\n")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	assigned, reported := opened.BackfillWorkstreamSlugs()
	if len(reported) != 0 {
		t.Errorf("the migration reported %+v on a workbench it could repair", reported)
	}
	if len(assigned) != 2 {
		t.Fatalf("the migration assigned %d slugs, wanted one per workstream", len(assigned))
	}
	if assigned[0].Slug != "portfolio-work" || assigned[1].Slug != "portfolio-work-2" {
		t.Errorf("the migration assigned %q and %q, wanted portfolio-work and its counting suffix", assigned[0].Slug, assigned[1].Slug)
	}
	reopened, err := Open(root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	findings, err := reopened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("check reported %+v after the repair", findings)
	}
}

// TestAWorkstreamDeletionCountsTheLiveCardsAlone asserts the population the
// deletion refusal reads, which is the reading this card makes of the format's
// own sentence rather than a quotation of it.
func TestAWorkstreamDeletionCountsTheLiveCardsAlone(t *testing.T) {
	root := newFixture(t)
	writeWorkstream(t, root, "f00000000001", "title: Portfolio work\nslug: portfolio\nstatus: active\nordinal: 1\n")
	edit(t, root, "state: ready", "state: ready\nworkstreams:\n  - f00000000001")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := opened.WorkstreamReferenced("f00000000001", "portfolio"); err == nil {
		t.Error("a live card lists the workstream and the scan reported nothing")
	}
	live := filepath.Join(root, CardsDir, "c00000000001")
	archived := filepath.Join(root, ArchiveDir, CardsDir, "c00000000001")
	if err := os.MkdirAll(filepath.Dir(archived), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Rename(live, archived); err != nil {
		t.Fatalf("archive the card: %v", err)
	}
	if err := opened.WorkstreamReferenced("f00000000001", "portfolio"); err != nil {
		t.Errorf("only an archived card lists the workstream and the scan refused: %v", err)
	}
}
