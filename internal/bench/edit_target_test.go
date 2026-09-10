package bench

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestAnchorPathOfReportsNoAnchorForAKindOutsideTheGrammar holds the join
// ResolveEditTarget's fourth arm rests on. dinah-467.
//
// The false second answer is what keeps a kind declaring no anchor from
// joining an empty filename onto the entity's directory and handing a
// directory back where a file was asked for, which is the defect this card
// fixed for the one kind that had it. The seven kinds below are the ones
// ResolveReference can answer, and a true answer for every one of them is
// what makes that arm unreachable for any reference a reader can type.
//
// The wanted anchor filenames are the format's own constants rather than
// AnchorOf's answers, since reading the function under test for the
// expectation would assert only that it agrees with itself.
// TestEveryKindsAnchorIsNamed in fields_test.go holds AnchorOf to the same
// set, and this test holds the join beside it.
func TestAnchorPathOfReportsNoAnchorForAKindOutsideTheGrammar(t *testing.T) {
	path, declared := AnchorPathOf(&EntityRef{Kind: "frobnicate", Dir: "/somewhere"})
	if declared {
		t.Errorf("a kind the grammar does not name declares an anchor, at %q", path)
	}
	if path != "" {
		t.Errorf("a kind the grammar does not name answers the path %q, wanted the empty string", path)
	}

	anchors := map[string]string{
		KindWorkbench:  WorkbenchAnchor,
		KindColumn:     ColumnAnchor,
		KindCard:       CardAnchor,
		KindComment:    CommentAnchor,
		KindItem:       ItemAnchor,
		KindAttachment: AttachmentAnchor,
		KindWorkstream: WorkstreamAnchor,
	}
	if len(anchors) != 7 {
		t.Fatalf("this test declares %d kinds, wanted the seven ResolveReference answers", len(anchors))
	}
	for kind, anchor := range anchors {
		dir := filepath.Join("/somewhere", kind)
		got, declared := AnchorPathOf(&EntityRef{Kind: kind, Dir: dir})
		if !declared {
			t.Errorf("%s declares no anchor, so ResolveEditTarget would refuse a reference naming one", kind)
			continue
		}
		if want := filepath.Join(dir, anchor); got != want {
			t.Errorf("%s answers %q, wanted %q", kind, got, want)
		}
		if !strings.HasSuffix(got, anchor) {
			t.Errorf("%s answers %q, which does not end in its own anchor %q", kind, got, anchor)
		}
	}
}
