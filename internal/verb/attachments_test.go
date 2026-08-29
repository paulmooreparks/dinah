package verb

import (
	"os"
	"path/filepath"
	"testing"

	"dinah/internal/bench"
)

// TestAnAttachmentPublishesThePathOfItsPayload asserts that every read
// reporting an attachment says where its bytes are, and that the path it says
// resolves to the file the attachment wraps (dinah-334 AC-1, AC-2).
//
// The assertion is what the path IS rather than that the key is present. The
// field is optional in the wire format, so an implementation may legitimately
// omit it, and a check for its presence would pass against one that published
// a path pointing nowhere, which is the interesting way to be wrong.
func TestAnAttachmentPublishesThePathOfItsPayload(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card carrying bytes")
	h.attach(ref, "notes.txt", "the bytes")

	detail, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref})
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if len(detail.Attachments) != 1 {
		t.Fatalf("wanted one attachment, got %d", len(detail.Attachments))
	}
	view := detail.Attachments[0]
	if !filepath.IsAbs(view.Path) {
		t.Errorf("the published path is not absolute: %q", view.Path)
	}
	if filepath.Base(view.Path) != "notes.txt" {
		t.Errorf("the published path does not end at the attachment's own filename: %q", view.Path)
	}
	body, err := os.ReadFile(view.Path)
	if err != nil {
		t.Fatalf("the published path does not open: %v", err)
	}
	if string(body) != "the bytes" {
		t.Errorf("the published path opens the wrong file, it holds %q", string(body))
	}

	// The reference and the path are two ways to the same file, and a read
	// that let them disagree would send a client to one attachment and a
	// person to another.
	resolved, err := h.library.Bench.ResolvePath(view.Ref + "/" + bench.PayloadDir)
	if err != nil {
		t.Fatalf("resolve %s: %v", view.Ref, err)
	}
	if resolved != view.Path {
		t.Errorf("the reference resolves to %q and the published path is %q", resolved, view.Path)
	}
}

// TestAnUnreadablePayloadEmptiesOnePathAndNoOther asserts the degradation
// Decision 1 asks for: an attachment whose payload will not read reports no
// path, the read still succeeds, and every other attachment of the same
// entity still carries its own path (dinah-334 AC-1).
//
// A caller asking about attachments when exactly this pathology is what they
// are chasing must not be answered with a refusal, and must not have the
// listing blanked out by the one broken member.
func TestAnUnreadablePayloadEmptiesOnePathAndNoOther(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card whose first attachment is broken")
	h.attach(ref, "broken.txt", "these bytes go away")
	h.attach(ref, "intact.txt", "these bytes stay")

	attachments, err := bench.Attachments(h.card(ref).Dir)
	if err != nil {
		t.Fatalf("attachments: %v", err)
	}
	if len(attachments) != 2 {
		t.Fatalf("wanted two attachments, got %d", len(attachments))
	}
	if err := os.RemoveAll(filepath.Join(attachments[0].Dir, bench.PayloadDir)); err != nil {
		t.Fatalf("remove the payload: %v", err)
	}

	detail, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref})
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if len(detail.Attachments) != 2 {
		t.Fatalf("a broken payload cost the listing a row: wanted two, got %d", len(detail.Attachments))
	}
	if detail.Attachments[0].Path != "" {
		t.Errorf("the broken attachment published %q, wanted no path", detail.Attachments[0].Path)
	}
	if detail.Attachments[0].Filename != "broken.txt" {
		t.Errorf("the broken attachment lost its other fields, filename is %q", detail.Attachments[0].Filename)
	}
	if detail.Attachments[1].Path == "" {
		t.Error("the intact attachment lost its path because a sibling was broken")
	}
}

// TestACommentPublishesItsOwnAttachments asserts that a card's detail carries
// each comment's attachments, addressed against the comment rather than
// against the card (dinah-334 AC-3).
//
// The reference is the half worth guarding. Composing a comment's attachment
// against the card's own reference produces an address that resolves, to a
// different attachment or to nothing, so the check resolves what the view
// prints instead of reading it.
func TestACommentPublishesItsOwnAttachments(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card whose comment carries bytes")
	h.attach(ref, "on-the-card.txt", "the card's own bytes")
	h.comment(ref, "a thought with a file under it")
	h.attach(ref+"/"+bench.CommentsDir+"/1", "on-the-comment.txt", "the comment's own bytes")

	detail, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref})
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if len(detail.Comments) != 1 {
		t.Fatalf("wanted one comment, got %d", len(detail.Comments))
	}
	below := detail.Comments[0].Attachments
	if len(below) != 1 {
		t.Fatalf("wanted the comment's one attachment, got %d", len(below))
	}
	if want := ref + "/comments/1/attachments/1"; below[0].Ref != want {
		t.Errorf("the comment's attachment is addressed %q, wanted %q", below[0].Ref, want)
	}
	resolved, err := h.library.Bench.ResolvePath(below[0].Ref + "/" + bench.PayloadDir)
	if err != nil {
		t.Fatalf("resolve %s: %v", below[0].Ref, err)
	}
	if resolved != below[0].Path {
		t.Errorf("the reference resolves to %q and the published path is %q", resolved, below[0].Path)
	}
	body, err := os.ReadFile(below[0].Path)
	if err != nil {
		t.Fatalf("the comment's attachment does not open: %v", err)
	}
	if string(body) != "the comment's own bytes" {
		t.Errorf("the comment's attachment opens the card's file instead: %q", string(body))
	}
}

// TestACountIsTakenWithoutOpeningAnAnchor asserts that the counts the
// many-entity reads carry come from a directory read alone, by breaking every
// anchor of a card's attachments collection and watching the count survive
// (dinah-334 AC-4).
//
// A count taken by listing the attachments would fall to zero here, since
// Attachments skips a member whose anchor will not read. The count is what
// tells a sidebar whether a row needs a child, so it has to answer from the
// collection rather than from its members.
func TestACountIsTakenWithoutOpeningAnAnchor(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card whose anchors are broken")
	h.attach(ref, "first.txt", "one")
	h.attach(ref, "second.txt", "two")

	card := h.card(ref)
	if got := bench.CountAttachments(card.Dir); got != 2 {
		t.Fatalf("the count before the break is %d, wanted 2", got)
	}
	for _, id := range bench.ListIDs(filepath.Join(card.Dir, bench.AttachmentsDir)) {
		anchor := filepath.Join(card.Dir, bench.AttachmentsDir, id, bench.AttachmentAnchor)
		if err := os.Remove(anchor); err != nil {
			t.Fatalf("break %s: %v", anchor, err)
		}
	}
	listed, err := bench.Attachments(card.Dir)
	if err != nil {
		t.Fatalf("attachments: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("the break did not reach the listing, which still reports %d", len(listed))
	}
	if got := bench.CountAttachments(card.Dir); got != 2 {
		t.Errorf("the count reads the anchors: got %d, wanted 2", got)
	}
	h.reopen()
	if got := h.library.view(h.card(ref)).AttachmentCount; got != 2 {
		t.Errorf("the card view reports %d attachments, wanted 2", got)
	}
}

// TestTheWorkbenchAndItsColumnsCountTheirOwnAttachments asserts that status
// carries the workbench's own count and each column view carries its own,
// neither borrowing the other's and neither counting what a card holds
// (dinah-334 AC-5).
func TestTheWorkbenchAndItsColumnsCountTheirOwnAttachments(t *testing.T) {
	h := newHarness(t)
	ref := h.readyAt("a card standing in intake", "a00000000001")
	h.attach("workbench", "on-the-workbench.txt", "one")
	h.attach("workbench", "also-on-the-workbench.txt", "two")
	h.attach("intake", "on-the-column.txt", "three")
	h.attach(ref, "on-the-card.txt", "four")

	status, err := h.library.Status(&Request{Verb: "status", Actor: "alka"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.AttachmentCount != 2 {
		t.Errorf("status reports %d attachments on the workbench, wanted 2", status.AttachmentCount)
	}
	counted := map[string]int{}
	for _, column := range status.Columns {
		counted[column.ID] = column.AttachmentCount
	}
	if counted["a00000000001"] != 1 {
		t.Errorf("intake reports %d attachments, wanted 1", counted["a00000000001"])
	}
	if counted["a00000000002"] != 0 {
		t.Errorf("a column with nothing attached reports %d, wanted 0", counted["a00000000002"])
	}
}

// TestAttachmentsReadsEveryKindTheGrammarMounts asserts that the new read
// answers for all four kinds that can carry an attachment, names each kind
// and its reference the way the resolver does, and answers an entity of a
// kind mounting nothing with an empty list rather than a refusal (dinah-334
// AC-6, AC-7, AC-8, AC-9).
func TestAttachmentsReadsEveryKindTheGrammarMounts(t *testing.T) {
	h := newHarness(t)
	ref := h.readyAt("a card standing in intake", "a00000000001")
	h.comment(ref, "a thought")
	h.attach("workbench", "on-the-workbench.txt", "one")
	h.attach("intake", "on-the-column.txt", "two")
	h.attach(ref, "on-the-card.txt", "three")
	h.attach(ref+"/"+bench.CommentsDir+"/1", "on-the-comment.txt", "four")
	writeItem(t, h.card(ref).Dir, "a criterion", 1)
	h.reopen()

	// AC-9 rests on these two kinds mounting nothing, so the premise is
	// asserted rather than assumed: a grammar that gave either one an
	// attachments collection would make the empty answer below wrong.
	for _, kind := range []string{bench.KindItem, bench.KindAttachment} {
		if mounts := bench.Contains(kind); len(mounts) != 0 {
			t.Fatalf("%s mounts %d collections, and this test's premise is that it mounts none", kind, len(mounts))
		}
	}

	cases := []struct {
		name     string
		ref      string
		kind     string
		wantRef  string
		filename string
	}{
		{name: "the workbench named by nothing at all", ref: "", kind: bench.KindWorkbench, wantRef: "workbench", filename: "on-the-workbench.txt"},
		{name: "the workbench named as workbench", ref: "workbench", kind: bench.KindWorkbench, wantRef: "workbench", filename: "on-the-workbench.txt"},
		{name: "the workbench named as a dot", ref: ".", kind: bench.KindWorkbench, wantRef: "workbench", filename: "on-the-workbench.txt"},
		{name: "a column", ref: "intake", kind: bench.KindColumn, wantRef: "intake", filename: "on-the-column.txt"},
		{name: "a card", ref: ref, kind: bench.KindCard, wantRef: ref, filename: "on-the-card.txt"},
		{name: "a comment", ref: ref + "/comments/1", kind: bench.KindComment, wantRef: ref + "/comments/1", filename: "on-the-comment.txt"},
		{name: "a checklist item, which mounts nothing", ref: ref + "/checklist/1", kind: bench.KindItem, wantRef: ref + "/checklist/1"},
		{name: "an attachment, which mounts nothing", ref: ref + "/attachments/1", kind: bench.KindAttachment, wantRef: ref + "/attachments/1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			listing, err := h.library.Attachments(&Request{Verb: "attachments", Actor: "alka", Ref: c.ref})
			if err != nil {
				t.Fatalf("attachments %q: %v", c.ref, err)
			}
			if listing.Kind != c.kind {
				t.Errorf("kind: wanted %s, got %s", c.kind, listing.Kind)
			}
			if listing.Ref != c.wantRef {
				t.Errorf("ref: wanted %s, got %s", c.wantRef, listing.Ref)
			}
			if listing.Attachments == nil {
				t.Fatal("the attachments list is nil, and an entity carrying none reports an empty list")
			}
			if c.filename == "" {
				if len(listing.Attachments) != 0 {
					t.Fatalf("wanted no attachment, got %d", len(listing.Attachments))
				}
				return
			}
			if len(listing.Attachments) != 1 {
				t.Fatalf("wanted one attachment, got %d", len(listing.Attachments))
			}
			view := listing.Attachments[0]
			if view.Filename != c.filename {
				t.Errorf("filename: wanted %s, got %s", c.filename, view.Filename)
			}
			if want := c.wantRef + "/attachments/1"; view.Ref != want {
				t.Errorf("the attachment is addressed %q, wanted %q", view.Ref, want)
			}
			resolved, err := h.library.Bench.ResolvePath(view.Ref + "/" + bench.PayloadDir)
			if err != nil {
				t.Fatalf("resolve %s: %v", view.Ref, err)
			}
			if resolved != view.Path {
				t.Errorf("the reference resolves to %q and the published path is %q", resolved, view.Path)
			}
		})
	}
}
