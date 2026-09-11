package bench

import (
	"testing"
)

// TestAWorkstreamIsPrintedInTheSpellingTheGrammarTakes asserts dinah-454 AC-6:
// a workstream's printed address resolves, and both spellings reach the
// commands that take a workstream and nothing else.
//
// The two halves are one criterion because either alone strands the reader.
// Before this card the listing printed the bare handle, which `dinah contents`
// refuses, while `dinah workstream get` refused the prefixed spelling the
// containment tree prints, so each spelling worked with one half of the
// surface and neither worked with both.
//
// The doubled prefix is the arm that keeps the tolerance honest. Nothing
// prints `workstream/workstream/<slug>`, so accepting it would mean the strip
// runs more than once and a caller could nest the prefix as deep as they like.
func TestAWorkstreamIsPrintedInTheSpellingTheGrammarTakes(t *testing.T) {
	root := newFixture(t)
	writeWorkstream(t, root, "f00000000001",
		"title: Portfolio work\nslug: portfolio\nstatus: active\nordinal: 1\n")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	workstream, gotErr10 := opened.WorkstreamByRef("portfolio")
	if gotErr10 != nil {
		t.Fatalf("WorkstreamByRef: %v", gotErr10)
	}
	if workstream == nil {
		t.Fatal("the fixture workstream does not resolve, so nothing below is asserted")
	}

	if got, want := workstream.Ref(), WorkstreamRefPrefix+"portfolio"; got != want {
		t.Errorf("the workstream is printed as %q, wanted %q, which is the spelling the reference grammar takes", got, want)
	}
	entity, err := opened.ResolveEntity(workstream.Ref())
	if err != nil {
		t.Fatalf("the printed spelling %q resolves to nothing: %v", workstream.Ref(), err)
	}
	if entity.Kind != KindWorkstream || entity.ID != workstream.ID {
		t.Errorf("%q resolves to the %s %s, wanted the workstream %s", workstream.Ref(), entity.Kind, entity.ID, workstream.ID)
	}

	for _, spelling := range []string{"portfolio", WorkstreamRefPrefix + "portfolio", workstream.ID} {
		found, gotErr9 := opened.WorkstreamByRef(spelling)
		if gotErr9 != nil {
			t.Fatalf("WorkstreamByRef: %v", gotErr9)
		}
		if found == nil {
			t.Errorf("the commands that take a workstream refuse %q, and a reader who read that off a screen has nowhere to go", spelling)
			continue
		}
		if found.ID != workstream.ID {
			t.Errorf("%q resolves to the workstream %s rather than to %s", spelling, found.ID, workstream.ID)
		}
	}
	doubled := WorkstreamRefPrefix + WorkstreamRefPrefix + "portfolio"
	got8, gotErr8 := opened.WorkstreamByRef(doubled)
	if gotErr8 != nil {
		t.Fatalf("WorkstreamByRef: %v", gotErr8)
	}
	if found := got8; found != nil {
		t.Errorf("%q resolves to the workstream %s, and no surface prints that spelling, so exactly one prefix is stripped", doubled, found.ID)
	}
}
