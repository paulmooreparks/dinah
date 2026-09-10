package bench

import (
	"path/filepath"
	"strconv"
	"testing"
)

// archivedHalfRow is one row of dinah-461 section 3's worked table: a
// reference, and the directory the fixture actually archived the entity to.
//
// The expected path is recorded by the fixture as it archives, taken from
// ArchiveTarget's own answer rather than composed from a layout constant here,
// so a change to the mirror's layout moves both sides of the comparison
// together instead of turning this into a second statement of where the
// archive sits.
type archivedHalfRow struct {
	// name is what a failure calls this row.
	name string
	// ref is the reference driven through ResolvePathIn.
	ref string
	// want is the path the fixture wrote the entity to.
	want string
}

// archivedHalfFixture is one workbench carrying every shape section 3's table
// names, and the paths its own archiving produced.
type archivedHalfFixture struct {
	root string
	rows []archivedHalfRow
}

// archiveDir moves an entity's directory into the archive mirror the way a
// structural act's sixth step does, and answers where it went.
func archiveDir(t *testing.T, dir string) string {
	t.Helper()
	target, err := ArchiveEntity(dir)
	if err != nil {
		t.Fatalf("archive %s: %v", dir, err)
	}
	return target
}

// TestTheArchivedHalfIsReadAtTheDeepestCollectionStep is dinah-461 AC-3.
//
// The table is a declared slice and the run fails when it holds fewer than ten
// rows, so a row deleted to make a failure go away stops the run rather than
// shrinking the subject set in silence.
//
// Section 3's own table writes every row against `pb-1`, and two of its rows
// need that card archived while four need it live. One workbench cannot hold
// both, so the archived-head rows are driven against a second card of the same
// fixture. What is under test is the shape of each row rather than the name in
// it.
//
// The eleventh row of that table is the workbench, which resolves nothing and
// is refused ahead of the walk, so it is pinned by the refusal checks in
// cmd/dinah rather than here. Ten is the whole of what this test can drive
// rather than a row quietly dropped.
func TestTheArchivedHalfIsReadAtTheDeepestCollectionStep(t *testing.T) {
	fixture := buildArchivedHalfFixture(t)
	if len(fixture.rows) < 10 {
		t.Fatalf("the table holds %d rows and section 3's resolving rows are ten", len(fixture.rows))
	}
	opened, err := Open(fixture.root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	for _, row := range fixture.rows {
		got, err := opened.ResolvePathIn(ArchivedHalf, row.ref)
		if err != nil {
			t.Errorf("%s: ResolvePathIn refuses %q with %v", row.name, row.ref, err)
			continue
		}
		want, err := filepath.Abs(row.want)
		if err != nil {
			t.Fatalf("%s: abs %s: %v", row.name, row.want, err)
		}
		if got != want {
			t.Errorf("%s: %q under the flag answers\n  %s\nand the fixture archived it to\n  %s", row.name, row.ref, got, want)
		}
	}
}

// TestAPositionUnderTheFlagCountsTheMirrorsOwnMembers is the positional half of
// dinah-461 AC-3, which the table above cannot carry because it turns on two
// answers rather than one.
//
// Asserting only that the flagged answer is right would pass against a build
// where both halves happened to hold the entity at position one, so the two
// answers are asserted to differ as well as each being asserted right.
func TestAPositionUnderTheFlagCountsTheMirrorsOwnMembers(t *testing.T) {
	root := newFixture(t)
	card := filepath.Join(root, CardsDir, "c00000000001")
	// Three comments in a known order: the first two are archived, so the
	// mirror holds them at positions one and two, and the third stays live at
	// position one of the live half.
	writeComment(t, root, "d00000000001", "2026-08-17T09:00:00Z", 1, "the first archived comment")
	writeComment(t, root, "d00000000002", "2026-08-17T09:01:00Z", 2, "the second archived comment")
	writeComment(t, root, "d00000000003", "2026-08-17T09:02:00Z", 3, "the live comment")
	firstArchived := archiveDir(t, filepath.Join(card, CommentsDir, "d00000000001"))
	archiveDir(t, filepath.Join(card, CommentsDir, "d00000000002"))

	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	flagged, err := opened.ResolvePathIn(ArchivedHalf, "fx-1/comments/1")
	if err != nil {
		t.Fatalf("the flagged read refuses: %v", err)
	}
	live, err := opened.ResolvePathIn(LiveHalf, "fx-1/comments/1")
	if err != nil {
		t.Fatalf("the unflagged read refuses: %v", err)
	}
	wantFlagged, err := filepath.Abs(filepath.Join(firstArchived, CommentAnchor))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	wantLive, err := filepath.Abs(filepath.Join(card, CommentsDir, "d00000000003", CommentAnchor))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if flagged != wantFlagged {
		t.Errorf("fx-1/comments/1 under the flag answers\n  %s\nand the first archived comment is at\n  %s", flagged, wantFlagged)
	}
	if live != wantLive {
		t.Errorf("fx-1/comments/1 without the flag answers\n  %s\nand the live comment is at\n  %s", live, wantLive)
	}
	if flagged == live {
		t.Error("both halves answer fx-1/comments/1 with the same path, so the position is not counted per half")
	}
}

// buildArchivedHalfFixture writes one workbench carrying every shape section
// 3's table names and records where each archived entity went.
func buildArchivedHalfFixture(t *testing.T) archivedHalfFixture {
	t.Helper()
	root := newFixture(t)
	fixture := archivedHalfFixture{root: root}
	add := func(name, ref, want string) {
		fixture.rows = append(fixture.rows, archivedHalfRow{name: name, ref: ref, want: want})
	}

	// A second card, archived whole, carries the two rows whose head is the
	// reference's deepest collection step.
	write(t, filepath.Join(root, CardsDir, "c00000000002", CardAnchor),
		"---\ntitle: The archived card\nnumber: 2\ncolumn: b00000000001\nstate: ready\n---\nFraming.\n")
	write(t, filepath.Join(root, CardsDir, "c00000000002", JournalName), cleanJournal)
	archivedCard := archiveDir(t, filepath.Join(root, CardsDir, "c00000000002"))
	add("an archived card by its bare head", "fx-2", filepath.Join(archivedCard, CardAnchor))
	add("an archived card's journal", "fx-2/journal", filepath.Join(archivedCard, JournalName))

	// The live card carries the rows whose deepest step is below its head.
	card := filepath.Join(root, CardsDir, "c00000000001")
	writeComment(t, root, "d00000000001", "2026-08-17T09:00:00Z", 1, "the archived comment")
	archivedComment := archiveDir(t, filepath.Join(card, CommentsDir, "d00000000001"))
	add("a comment in the card's mirror", "fx-1/comments/1", filepath.Join(archivedComment, CommentAnchor))
	add("the card's archived comments collection", "fx-1/comments", filepath.Dir(archivedComment))

	// An attachment below a LIVE comment, archived in the comment's own
	// mirror, is the row that fails when the flag is honoured at every step
	// rather than at the deepest one.
	writeComment(t, root, "d00000000002", "2026-08-17T09:01:00Z", 2, "the live comment")
	liveComment := filepath.Join(card, CommentsDir, "d00000000002")
	writeAttachmentIn(t, liveComment, "e00000000001", 1)
	archivedBelowComment := archiveDir(t, filepath.Join(liveComment, AttachmentsDir, "e00000000001"))
	add("an attachment below a live comment", "fx-1/comments/1/attachments/1", filepath.Join(archivedBelowComment, AttachmentAnchor))

	// An archived attachment of the card, reached through its payload, which
	// is the one reference ResolvePath answers and ResolveReference refuses.
	writeAttachmentIn(t, card, "e00000000002", 1)
	write(t, filepath.Join(card, AttachmentsDir, "e00000000002", PayloadDir, "e00000000002.txt"), "payload bytes\n")
	archivedAttachment := archiveDir(t, filepath.Join(card, AttachmentsDir, "e00000000002"))
	add("an archived attachment's payload", "fx-1/attachments/1/payload", filepath.Join(archivedAttachment, PayloadDir, "e00000000002.txt"))

	// A live column carrying an archived attachment, and an archived column
	// reached by its own bare head.
	column := filepath.Join(root, ColumnsDir, "b00000000001")
	writeAttachmentIn(t, column, "e00000000003", 1)
	archivedBelowColumn := archiveDir(t, filepath.Join(column, AttachmentsDir, "e00000000003"))
	add("an attachment below a live column", "only/attachments/1", filepath.Join(archivedBelowColumn, AttachmentAnchor))

	write(t, filepath.Join(root, ColumnsDir, "b00000000002", ColumnAnchor),
		"---\ntitle: Retired\nslug: retired\nkind: work\n---\nColumn text.\n")
	archivedColumn := archiveDir(t, filepath.Join(root, ColumnsDir, "b00000000002"))
	add("an archived column by its bare head", "retired", filepath.Join(archivedColumn, ColumnAnchor))

	// An archived attachment of the workbench itself, addressed below the
	// slug, which is the form the containment walk prints for one.
	writeAttachmentIn(t, root, "e00000000004", 1)
	archivedBelowBench := archiveDir(t, filepath.Join(root, AttachmentsDir, "e00000000004"))
	add("an attachment below the workbench", "fx/attachments/1", filepath.Join(archivedBelowBench, AttachmentAnchor))

	// An archived workstream by its own bare head.
	writeWorkstream(t, root, "f00000000001", "title: Effort\nslug: effort\nstatus: active\nordinal: 1\n")
	archivedWorkstream := archiveDir(t, filepath.Join(root, WorkstreamsDir, "f00000000001"))
	add("an archived workstream", WorkstreamRefPrefix+"effort", archivedWorkstream)

	return fixture
}

// writeAttachmentIn puts an attachment below any entity that mounts an
// attachments collection, which check_test.go's writeAttachment does for the
// fixture card alone.
func writeAttachmentIn(t *testing.T, holder, id string, ordinal int) {
	t.Helper()
	fm := NewFrontmatter()
	fm.Set("filename", id+".txt")
	fm.Set("provenance", "alka")
	if ordinal > 0 {
		fm.Set(OrdinalField, strconv.Itoa(ordinal))
	}
	write(t, filepath.Join(holder, AttachmentsDir, id, AttachmentAnchor), fm.Render(""))
}
