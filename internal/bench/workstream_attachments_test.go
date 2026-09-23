package bench

import (
	"errors"
	"path/filepath"
	"testing"

	"dinah/internal/contract"
)

// TestTheContainmentTableGivesAWorkstreamOneMount asserts the mount dinah-583
// adds, and the three properties around it that a later simplification would
// take away. The workstream mounts the attachments collection and nothing
// else; the entity-kind set is unchanged by the new table key, because
// EntityKinds already seeded the workstream; and AnchorOf still answers
// workstream.md from its own switch arm rather than from the table, since no
// mount names the workstream as its kind and the loop below that switch
// therefore cannot reach it.
//
// The acyclicity the table's own comment rests five recursive walks on is
// asserted too, because the new key is the first one added since that comment
// was written.
//
// Arming: removing KindWorkstream from containment reddens the mount
// assertions; giving it a comments mount as well reddens the exactly-one
// assertion; deleting the KindWorkstream arm of AnchorOf's switch reddens the
// anchor assertion while leaving everything else green.
func TestTheContainmentTableGivesAWorkstreamOneMount(t *testing.T) {
	mounts := Contains(KindWorkstream)
	if len(mounts) != 1 {
		t.Fatalf("a workstream mounts %d collections, wanted exactly one: %+v", len(mounts), mounts)
	}
	mount := mounts[0]
	if mount.Dir != AttachmentsDir {
		t.Errorf("the mount hangs under %q, wanted %q", mount.Dir, AttachmentsDir)
	}
	if mount.Kind != KindAttachment {
		t.Errorf("the mount holds %q, wanted %q", mount.Kind, KindAttachment)
	}
	if mount.Anchor != AttachmentAnchor {
		t.Errorf("the mount's anchor is %q, wanted %q", mount.Anchor, AttachmentAnchor)
	}
	if mount.NameField != "filename" {
		t.Errorf("the mount's name field is %q, wanted %q, which is what a name selector reads", mount.NameField, "filename")
	}
	if !mount.Stamped {
		t.Error("the mount is unstamped, so its members would carry no ordinal and a positional reference would not count in creation order")
	}
	if got, ok := MountOf(KindWorkstream, AttachmentsDir); !ok || got != mount {
		t.Errorf("MountOf answers %+v ok=%t, wanted the mount Contains reports", got, ok)
	}
	if _, ok := MountOf(KindWorkstream, CommentsDir); ok {
		t.Error("a workstream mounts a comments collection, and the mount was meant to open one collection rather than the grammar")
	}

	if got := AnchorOf(KindWorkstream); got != WorkstreamAnchor {
		t.Errorf("AnchorOf(KindWorkstream) answers %q, wanted %q from its own switch arm", got, WorkstreamAnchor)
	}
	wanted := map[string]bool{
		KindWorkbench:  true,
		KindColumn:     true,
		KindCard:       true,
		KindComment:    true,
		KindItem:       true,
		KindAttachment: true,
		KindWorkstream: true,
	}
	kinds := EntityKinds()
	if len(kinds) != len(wanted) {
		t.Errorf("EntityKinds answers %d kinds, wanted %d: %v", len(kinds), len(wanted), kinds)
	}
	for _, kind := range kinds {
		if !wanted[kind] {
			t.Errorf("EntityKinds answers %q, which this build does not declare", kind)
		}
		delete(wanted, kind)
	}
	for kind := range wanted {
		t.Errorf("EntityKinds does not answer %q", kind)
	}

	// The table is acyclic, which is the termination argument five recursive
	// readers of Contains carry instead of a depth bound or a visited set.
	// The walk below is the one place that argument is checked rather than
	// asserted, and it is bounded by the path it carries.
	visited := 0
	var walk func(kind string, path []string)
	walk = func(kind string, path []string) {
		visited++
		for _, seen := range path {
			if seen == kind {
				t.Fatalf("the containment table reaches %s from itself, through %v", kind, append(path, kind))
			}
		}
		for _, mount := range Contains(kind) {
			walk(mount.Kind, append(path, kind))
		}
	}
	for _, kind := range kinds {
		walk(kind, nil)
	}
	if visited == 0 {
		t.Fatal("the acyclicity walk visited nothing, so it asserted nothing")
	}
}

// TestAWorkstreamAttachmentResolvesAndComposesOneSpelling asserts the address
// dinah-583 opens, in both directions and in every spelling that reaches it.
//
// One entity gets one spelling: the identifier-headed reference resolves to
// the same attachment as the slug-headed one and composes the slug back,
// because Workstream.Ref prefers the slug. That is the property section 4 of
// the contract rests on, and it is what keeps the workstream clear of the
// two-spellings split the workbench's own attachments carry.
//
// The negatives are pinned beside the positives, because a mount that opened
// the whole grammar would pass every positive here. A comments segment below
// a workstream refuses unknown-path, and an unknown handle refuses
// unknown-workstream naming the handle alone rather than the whole tail.
//
// Arming: removing KindWorkstream's mount from the containment table reddens
// every positive; composing the reference from the caller's own head instead
// of from Workstream.Ref reddens the identifier-headed case; refusing the
// whole remainder instead of the handle reddens the unknown-handle case.
func TestAWorkstreamAttachmentResolvesAndComposesOneSpelling(t *testing.T) {
	root := newFixture(t)
	writeWorkstream(t, root, "f00000000001",
		"title: Portfolio work\nslug: portfolio\nstatus: active\nordinal: 1\n")
	writeWorkstream(t, root, "f00000000002",
		"title: Unslugged work\nstatus: active\nordinal: 2\n")
	writeWorkstream(t, root, "f00000000003",
		"title: Awkwardly named\nslug: workstream\nstatus: active\nordinal: 3\n")
	for _, id := range []string{"f00000000001", "f00000000002", "f00000000003"} {
		plantAttachment(t, filepath.Join(root, WorkstreamsDir, id, AttachmentsDir), "b00000000009")
	}
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	slugged := filepath.Join(root, WorkstreamsDir, "f00000000001", AttachmentsDir, "b00000000009")
	for _, spelling := range []string{
		"workstream/portfolio/attachments/1",
		"workstream/portfolio/attachments/b00000000009",
		"workstream/portfolio/attachments/evidence.txt",
		"workstream/f00000000001/attachments/1",
	} {
		entity, err := opened.ResolveEntity(spelling)
		if err != nil {
			t.Errorf("%q resolves to nothing: %v", spelling, err)
			continue
		}
		if entity.Kind != KindAttachment {
			t.Errorf("%q resolves to a %s, wanted an attachment", spelling, entity.Kind)
		}
		if entity.Dir != slugged {
			t.Errorf("%q resolves to %s, wanted %s", spelling, entity.Dir, slugged)
		}
		// One entity, one spelling, whichever spelling the caller typed.
		if want := "workstream/portfolio/attachments/1"; entity.Ref != want {
			t.Errorf("%q composes back as %q, wanted %q", spelling, entity.Ref, want)
		}
	}

	// A workstream carrying no slug composes the identifier form, because
	// Workstream.Ref falls back to it.
	unslugged, err := opened.ResolveEntity("workstream/f00000000002/attachments/1")
	if err != nil {
		t.Fatalf("the unslugged workstream's attachment resolves to nothing: %v", err)
	}
	if want := "workstream/f00000000002/attachments/1"; unslugged.Ref != want {
		t.Errorf("the unslugged workstream's attachment composes as %q, wanted %q", unslugged.Ref, want)
	}

	// The double-strip position: a workstream slugged `workstream` resolves
	// through the split rather than losing a prefix twice.
	awkward, err := opened.ResolveEntity("workstream/workstream/attachments/1")
	if err != nil {
		t.Fatalf("a workstream slugged `workstream` strands its attachment: %v", err)
	}
	if want := filepath.Join(root, WorkstreamsDir, "f00000000003", AttachmentsDir, "b00000000009"); awkward.Dir != want {
		t.Errorf("`workstream/workstream/attachments/1` resolves to %s, wanted %s", awkward.Dir, want)
	}

	// The collection itself is a collection, not an entity.
	entity, collection, err := opened.ResolveReference("workstream/portfolio/attachments")
	if err != nil {
		t.Fatalf("the collection resolves to nothing: %v", err)
	}
	if entity != nil {
		t.Errorf("the collection resolved to the %s %s, and a collection is not an entity", entity.Kind, entity.Ref)
	}
	if collection == nil {
		t.Fatal("the collection reference answered no collection")
	}
	if len(collection.Members) != 1 {
		t.Errorf("the collection carries %d members, wanted one", len(collection.Members))
	}
	if collection.Holder == nil || collection.Holder.Kind != KindWorkstream {
		t.Errorf("the collection hangs off %+v, wanted the workstream", collection.Holder)
	}

	for _, c := range []struct {
		ref    string
		name   string
		detail string
	}{
		{ref: "workstream/portfolio/comments/1", name: contract.UnknownPath, detail: "comments"},
		{ref: "workstream/nosuch/attachments/1", name: contract.UnknownWorkstream, detail: "nosuch"},
	} {
		_, err := opened.ResolveEntity(c.ref)
		var refusal *contract.Refusal
		if !errors.As(err, &refusal) {
			t.Errorf("%q answered %v, wanted the refusal %s", c.ref, err, c.name)
			continue
		}
		if refusal.Name != c.name {
			t.Errorf("%q refuses %s, wanted %s", c.ref, refusal.Name, c.name)
		}
		if refusal.Detail != c.detail {
			t.Errorf("%q refuses naming %q, wanted %q", c.ref, refusal.Detail, c.detail)
		}
	}

	// The bare and trailing-slash forms are unmoved by the split.
	bare, err := opened.ResolveEntity("workstream/portfolio")
	if err != nil {
		t.Fatalf("the bare workstream reference resolves to nothing: %v", err)
	}
	if bare.Kind != KindWorkstream {
		t.Errorf("the bare reference resolves to a %s, wanted a workstream", bare.Kind)
	}
	if _, err := opened.ResolveEntity("workstream/portfolio/"); err == nil {
		t.Error("`workstream/portfolio/` resolved, and a trailing slash names no workstream")
	}

	// ResolvePath reaches the two files below an attachment, and the bare
	// form still answers the workstream's directory rather than its anchor.
	payload, err := opened.ResolvePath("workstream/portfolio/attachments/1/payload")
	if err != nil {
		t.Fatalf("the payload path resolves to nothing: %v", err)
	}
	if filepath.Base(payload) != "evidence.txt" {
		t.Errorf("the payload path answers %s, wanted the payload file", payload)
	}
	dir, err := opened.ResolvePath("workstream/portfolio")
	if err != nil {
		t.Fatalf("the bare workstream path resolves to nothing: %v", err)
	}
	if dir != filepath.Join(root, WorkstreamsDir, "f00000000001") {
		t.Errorf("the bare workstream path answers %s, wanted the workstream's directory", dir)
	}
	anchor, err := opened.ResolveEditTarget("workstream/portfolio")
	if err != nil {
		t.Fatalf("the edit target resolves to nothing: %v", err)
	}
	if filepath.Base(anchor) != WorkstreamAnchor {
		t.Errorf("edit opens %s, wanted %s", anchor, WorkstreamAnchor)
	}
	// edit opens an attachment's anchor rather than its bytes, which is what
	// AnchorPathOf answers for every holder; the bytes are reached by the
	// payload path asserted above.
	below, err := opened.ResolveEditTarget("workstream/portfolio/attachments/1")
	if err != nil {
		t.Fatalf("the attachment's edit target resolves to nothing: %v", err)
	}
	if filepath.Base(below) != AttachmentAnchor {
		t.Errorf("edit on the attachment opens %s, wanted %s", below, AttachmentAnchor)
	}
	if _, err := opened.ResolveEditTarget("workstream/portfolio/attachments"); err == nil {
		t.Error("edit opened a whole collection, and a collection directory is not a file an editor can open")
	}
}
