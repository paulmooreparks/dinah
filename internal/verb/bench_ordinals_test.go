package verb

import (
	"os"
	"path/filepath"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// plantUnstampedEntity writes an anchor carrying no ordinal field into a
// collection, which is the state a crashed write or a hand edit leaves
// behind. It is the one defect this sweep exists to report.
func plantUnstampedEntity(t *testing.T, collection, id, anchor, body string) {
	t.Helper()
	fm := bench.NewFrontmatter()
	fm.Set("ts", "2026-08-17T09:00:00Z")
	fm.Set("author", "alka")
	if anchor == bench.AttachmentAnchor {
		fm.Set("filename", "planted.txt")
	}
	path := filepath.Join(collection, id, anchor)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(fm.Render(body)), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestCheckReportsAnUnstampedMemberBelowTheWorkbenchAndBelowAColumn asserts
// dinah-518/criteria/13's two runs.
//
// The clean run comes first and it is the one that arms the Stamped filter: a
// build sweeping the workbench's columns and cards collections, whose anchors
// carry no ordinal at all, reports one finding per column and one per card
// and fails here on the count even though it would still find a planted
// defect. The fixture carries more than one column and more than one card for
// exactly that reason.
//
// The planted run then places three unstamped members at the two levels a
// card-rooted sweep never reaches, and requires those three and no fourth.
func TestCheckReportsAnUnstampedMemberBelowTheWorkbenchAndBelowAColumn(t *testing.T) {
	h := newHarness(t)
	if len(h.library.Bench.Columns) < 2 {
		t.Fatalf("the fixture declares %d columns, and this run needs at least two", len(h.library.Bench.Columns))
	}
	first := h.library.Bench.Columns[0]
	second := h.library.Bench.Columns[1]

	cards := []string{h.add("the first card"), h.add("the second card")}
	for _, card := range cards {
		h.comment(card, "a remark")
		h.attach(card, "notes.txt", "bytes")
	}
	for _, spelling := range []string{first.Slug, second.Slug} {
		if response := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: spelling, Text: "a station note"}); response.Outcome != contract.OutcomeOK {
			t.Fatalf("comment on %q: %s %s", spelling, response.Outcome, response.Refusal)
		}
		h.reopen()
	}
	h.attach(first.Slug, "rubric.txt", "bytes")
	h.attach("workbench", "charter.txt", "bytes")

	clean := h.check()
	if len(clean) != 0 {
		t.Fatalf("a workbench written entirely by the verbs reports %d findings, wanted none: %+v", len(clean), clean)
	}

	// Three plants, one per site a card-rooted sweep does not reach.
	plantUnstampedEntity(t,
		filepath.Join(h.library.Bench.ColumnDir(first.ID), bench.CommentsDir),
		"e00000000001", bench.CommentAnchor, "a hand-made column comment")
	plantUnstampedEntity(t,
		filepath.Join(h.library.Bench.ColumnDir(second.ID), bench.AttachmentsDir),
		"f00000000001", bench.AttachmentAnchor, "")
	plantUnstampedEntity(t,
		filepath.Join(h.library.Bench.Root, bench.AttachmentsDir),
		"f00000000002", bench.AttachmentAnchor, "")
	h.reopen()

	planted := h.check()
	missing := 0
	for _, f := range planted {
		if f.Key == bench.FindingOrdinalMissing {
			missing++
			continue
		}
		t.Errorf("the planted run reports %s at %s, and this run expects only %s", f.Key, f.Path, bench.FindingOrdinalMissing)
	}
	if missing != 3 {
		t.Errorf("the planted run reports %d missing ordinals, wanted exactly 3: %+v", missing, planted)
	}
}
