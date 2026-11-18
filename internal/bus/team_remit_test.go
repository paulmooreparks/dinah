package bus

import (
	"context"
	"testing"
)

// A member's remit is kept as it joins, survives a move to another lead, and is in the member's row; a chat's tree is
// its members at any depth, never its own lead.
func TestARemitIsKeptAndATreeIsItsMembersAtAnyDepth(t *testing.T) {
	ctx := context.Background()
	b, _, lead, docs, parser, outside := teamBus(t)
	if _, err := b.SetLeadWithRemit(ctx, docs.ID, lead.ID, "Keep the docs right."); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetLeadWithRemit(ctx, parser.ID, docs.ID, "Fix the parser."); err != nil {
		t.Fatal(err)
	}
	if remit, err := b.Remit(ctx, docs.ID); err != nil || remit != "Keep the docs right." {
		t.Fatalf("the remit %q %v", remit, err)
	}
	members, err := b.TeamMembers(ctx, docs.ID)
	if err != nil || len(members) != 1 || members[0].Remit != "Fix the parser." {
		t.Fatalf("the members %+v %v", members, err)
	}
	for _, c := range []struct {
		root, session string
		want          bool
	}{{lead.ID, docs.ID, true}, {lead.ID, parser.ID, true}, {docs.ID, parser.ID, true}, {docs.ID, lead.ID, false}, {parser.ID, docs.ID, false}, {lead.ID, lead.ID, false}, {lead.ID, outside.ID, false}} {
		if got, err := b.InTree(ctx, c.root, c.session); err != nil || got != c.want {
			t.Fatalf("InTree(%s, %s) = %v %v", c.root, c.session, got, err)
		}
	}
	// Moving the member to another lead keeps what it is for.
	if _, err := b.SetLead(ctx, parser.ID, lead.ID); err != nil {
		t.Fatal(err)
	}
	if remit, _ := b.Remit(ctx, parser.ID); remit != "Fix the parser." {
		t.Fatalf("the moved member's remit is %q", remit)
	}
	if in, _ := b.InTree(ctx, docs.ID, parser.ID); in {
		t.Fatal("the moved member is still in its old lead's tree")
	}
}
