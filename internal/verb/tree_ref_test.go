package verb

import (
	"testing"
)

// TestTheContainmentWalkAndShowAgreeOnEveryItemReference asserts dinah-454
// AC-5: `dinah contents <card>` and `dinah show <card>` print the same
// reference for the same checklist item.
//
// The two are paired by identifier rather than by position, so a walk emitting
// the right references in the wrong order still fails. Comparing the two
// producers against each other rather than either against a literal is what
// lets this criterion survive a later change to the aliasing rule: the rule
// under guard is that one item has one address, not what that address spells.
func TestTheContainmentWalkAndShowAgreeOnEveryItemReference(t *testing.T) {
	h := newHarness(t)
	card := h.add("a card carrying a mixed checklist")
	h.file(card, "open_question", "the first question")
	h.file(card, "open_question", "the second question")
	h.file(card, "acceptance_criterion", "the criterion")
	h.item(card, "c00000000004", "kind: risk\nordinal: 4\n", "a risk")

	detail, _, err := h.library.Show(&Request{Verb: "show", Card: card})
	if err != nil {
		t.Fatalf("show %s: %v", card, err)
	}
	shown := map[string]string{}
	for _, view := range detail.Checklist {
		shown[view.ID] = view.Ref
	}
	if len(shown) != 4 {
		t.Fatalf("show drew %d items, wanted the four the fixture wrote", len(shown))
	}

	tree, err := h.library.Contents(&Request{Verb: "contents", Ref: card}, LevelAll)
	if err != nil {
		t.Fatalf("contents %s: %v", card, err)
	}
	walked := 0
	walkTree(tree.Root, func(node TreeNode) {
		ref, ok := shown[node.ID]
		if !ok {
			return
		}
		walked++
		if node.Ref != ref {
			t.Errorf("the item %s is drawn by contents as %q and by show as %q", node.ID, node.Ref, ref)
		}
		entity, err := h.library.Bench.ResolveEntity(node.Ref)
		if err != nil {
			t.Errorf("the item %s is drawn as %q, which resolves to nothing: %v", node.ID, node.Ref, err)
			return
		}
		if entity.ID != node.ID {
			t.Errorf("%q resolves to the item %s, and it is drawn on the row of %s", node.Ref, entity.ID, node.ID)
		}
	})
	if walked != 4 {
		t.Fatalf("the containment walk drew %d of the card's four items, so the pairing covered less than it claims", walked)
	}
}
