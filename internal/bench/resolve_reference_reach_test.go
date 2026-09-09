package bench

import (
	"path/filepath"
	"testing"
)

// TestResolveReferenceReachesAWorkstreamAndRefusesAnAttachmentPayload holds
// the sentence three comments in dinah-455 state about what the two resolvers
// reach. Those comments first said ResolvePath reaches two forms
// ResolveReference does not, "an attachment's payload and a workstream", and
// the workstream half was false: ResolveReference resolves a workstream in its
// third statement, before the head and rest are cut apart.
//
// The false conjunct rode inside a true compound, so the half that holds
// carried the half that does not past a reader, and nothing in the tree
// contradicted either. This test contradicts both halves directly rather than
// grepping for a wording, so a comment restating the claim in new words is
// still caught by the code disagreeing with it.
//
// Each of the three assertions below fails on its own: a resolver that stopped
// answering workstreams fails the first, a resolver that answered a payload
// fails the third, and a ResolvePath that stopped reaching one fails the
// second, which is the assertion that earns the payload half its place in the
// comments.
func TestResolveReferenceReachesAWorkstreamAndRefusesAnAttachmentPayload(t *testing.T) {
	root := newFixture(t)
	writeWorkstream(t, root, "f00000000001",
		"title: Portfolio work\nslug: portfolio\nstatus: active\nordinal: 1\n")
	writeAttachment(t, root, "a00000000001", 1)
	write(t, filepath.Join(root, CardsDir, "c00000000001", AttachmentsDir, "a00000000001", PayloadDir, "a00000000001.txt"), "payload bytes\n")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	workstreamRef := WorkstreamRefPrefix + "portfolio"
	entity, collection, err := opened.ResolveReference(workstreamRef)
	switch {
	case err != nil:
		t.Errorf("ResolveReference refuses %q with %v, and a comment claiming it does not reach a workstream would be right", workstreamRef, err)
	case collection != nil:
		t.Errorf("ResolveReference reads %q as a collection, and a workstream is one entity", workstreamRef)
	case entity == nil:
		t.Errorf("ResolveReference answers %q with neither an entity nor a collection", workstreamRef)
	case entity.Kind != KindWorkstream:
		t.Errorf("ResolveReference answers %q with a %s, wanted a %s", workstreamRef, entity.Kind, KindWorkstream)
	}

	payloadRef := "fx-1/" + AttachmentsDir + "/1/" + PayloadDir
	resolved, err := opened.ResolvePath(payloadRef)
	if err != nil {
		t.Fatalf("ResolvePath refuses %q with %v, so the payload half of the comments has nothing under it: %v", payloadRef, err, err)
	}
	if got := filepath.Base(resolved); got != "a00000000001.txt" {
		t.Errorf("ResolvePath answers %q with %q, wanted the payload file the attachment wraps", payloadRef, got)
	}
	if _, _, err := opened.ResolveReference(payloadRef); err == nil {
		t.Errorf("ResolveReference answers %q, and the comments say it refuses one because a payload file carries no anchor", payloadRef)
	}
}

// TestResolvePathAnswersACollectionWhereResolveEntityRefusesIt holds the
// clause resolveBelow's doc comment states about the two resolvers. That
// comment said ResolvePath and ResolveEntity are two readings of the pair
// resolveBelow returns, so both accept the same references. This card made
// that false: a reference naming a whole collection resolves to the
// collection's directory through ResolvePath and is refused by ResolveEntity,
// which is the addressing this card set out to separate.
//
// The claim is guarded here rather than in prose for the reason the test above
// gives, since a comment restating it in new words is still caught by the
// resolvers disagreeing with it.
func TestResolvePathAnswersACollectionWhereResolveEntityRefusesIt(t *testing.T) {
	root := newFixture(t)
	writeAttachment(t, root, "a00000000001", 1)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	collectionRef := "fx-1/" + AttachmentsDir
	resolved, err := opened.ResolvePath(collectionRef)
	if err != nil {
		t.Fatalf("ResolvePath refuses %q with %v, and the comment says it answers a reference naming a whole collection", collectionRef, err)
	}
	if got := filepath.Base(resolved); got != AttachmentsDir {
		t.Errorf("ResolvePath answers %q with %q, wanted the collection's own directory", collectionRef, got)
	}
	if _, err := opened.ResolveEntity(collectionRef); err == nil {
		t.Errorf("ResolveEntity answers %q, and the comment says it refuses a reference naming a whole collection", collectionRef)
	}
}
