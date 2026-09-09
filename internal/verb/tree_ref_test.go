package verb

import (
	"os"
	"path/filepath"
	"testing"

	"dinah/internal/bench"
)

// TestTheContainmentWalkAndShowAgreeOnEveryItemReference asserts dinah-454
// AC-5: `dinah contents <card>` and `dinah show <card>` print the same
// reference for the same checklist item.
//
// The two are paired by identifier rather than by position, so a walk emitting
// the right references in the wrong order still fails. Comparing the two
// producers against each other rather than either against a literal is what
// lets this criterion survive a later change to the addressing rule: the rule
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

// TestShowAddressesAnItemBelowADamagedOneTheWayTheWalkDoes asserts dinah-454
// AC-4's damaged-collection arm: an item standing after one whose anchor will
// not open is addressed identically by show and by contents, and that address
// reaches the item it is drawn on.
//
// The fixture is the one the clean cases cannot be: bench.Items skips an item
// it cannot load and the containment walk keeps it, so the two surfaces count
// different collections and every item after the damage is where
// they can disagree. The item is written whole and its anchor removed
// afterwards, because the collection must still list the directory for the
// walk to pass it.
//
// The two arms fail for different reasons and both are needed. The agreement
// arm catches one item drawn under two addresses, which is what a reader sees
// when the two screens are open together. The resolution arm catches the worse
// half on its own: an address that is not merely a second spelling but reaches
// a different item, which reads as a confident answer rather than as a defect.
func TestShowAddressesAnItemBelowADamagedOneTheWayTheWalkDoes(t *testing.T) {
	h := newHarness(t)
	card := h.add("a card whose first item will not open")
	damaged := h.item(card, "c00000000001", "kind: risk\nordinal: 1\n", "the item that stops opening")
	h.file(card, "open_question", "the question that survives")
	h.item(card, "c00000000003", "kind: risk\nordinal: 3\n", "a hand written risk")
	if err := os.Remove(filepath.Join(damaged, bench.ItemAnchor)); err != nil {
		t.Fatalf("remove the first item's anchor: %v", err)
	}
	h.reopen()

	detail, _, err := h.library.Show(&Request{Verb: "show", Card: card})
	if err != nil {
		t.Fatalf("show %s: %v", card, err)
	}
	shown := map[string]string{}
	for _, view := range detail.Checklist {
		shown[view.ID] = view.Ref
	}
	if len(shown) != 2 {
		t.Fatalf("show drew %d items, wanted the two that still open", len(shown))
	}
	if _, drawn := shown["c00000000003"]; !drawn {
		t.Fatalf("show did not draw the risk item at all, so this case asserts nothing about it")
	}

	tree, err := h.library.Contents(&Request{Verb: "contents", Ref: card}, LevelAll)
	if err != nil {
		t.Fatalf("contents %s: %v", card, err)
	}
	paired := 0
	walkTree(tree.Root, func(node TreeNode) {
		ref, ok := shown[node.ID]
		if !ok {
			return
		}
		paired++
		if node.Ref != ref {
			t.Errorf("the item %s stands after an item whose anchor will not open, and contents draws it as %q where show draws it as %q",
				node.ID, node.Ref, ref)
		}
		entity, err := h.library.Bench.ResolveEntity(ref)
		if err != nil {
			t.Errorf("show draws the item %s as %q, which resolves to nothing: %v", node.ID, ref, err)
			return
		}
		if entity.ID != node.ID {
			t.Errorf("show draws the item %s as %q, and that address reaches the item %s instead", node.ID, ref, entity.ID)
		}
	})
	if paired != 2 {
		t.Fatalf("the containment walk drew %d of the two items show drew, so the pairing covered less than it claims", paired)
	}
}
