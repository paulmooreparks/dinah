package verb

import (
	"testing"

	"dinah/internal/bench"
)

// TestACommentViewCarriesAReferenceThatResolves asserts dinah-454 AC-3: the
// machine surface carries a comment's address, and that address is the
// comment's position in the collection as it stands rather than the ordinal
// its anchor was stamped with.
//
// The fixture deletes the first of three comments, which is what makes the two
// numbers disagree: NextOrdinal hands out highest-plus-one, so the survivors
// keep the ordinals 2 and 3 while their positions become 1 and 2. Each
// reference is checked by handing it back to the resolver rather than by
// comparing it against a string this test composed, so the guard cannot
// recompute its expectation from the code it is guarding.
func TestACommentViewCarriesAReferenceThatResolves(t *testing.T) {
	h := newHarness(t)
	card := h.add("a card that collects comments")
	h.comment(card, "the first thought")
	h.comment(card, "the second thought")
	h.comment(card, "the third thought")
	h.remove(card + "/comments/1")

	detail, _, err := h.library.Show(&Request{Verb: "show", Card: card})
	if err != nil {
		t.Fatalf("show %s: %v", card, err)
	}
	if len(detail.Comments) != 2 {
		t.Fatalf("the card carries %d comments, wanted the two that survived the delete", len(detail.Comments))
	}
	seen := map[string]bool{}
	for _, view := range detail.Comments {
		if view.Ref == "" {
			t.Errorf("the comment %s carries no reference, so nothing on the row tells a reader how to name it", view.ID)
			continue
		}
		if seen[view.Ref] {
			t.Errorf("two comments are both addressed %q, so one of them is unreachable", view.Ref)
		}
		seen[view.Ref] = true
		entity, err := h.library.Bench.ResolveEntity(view.Ref)
		if err != nil {
			t.Errorf("the comment %s is addressed %q, which resolves to nothing: %v", view.ID, view.Ref, err)
			continue
		}
		if entity.Kind != bench.KindComment {
			t.Errorf("%q resolves to a %s rather than a comment", view.Ref, entity.Kind)
		}
		if entity.ID != view.ID {
			t.Errorf("%q resolves to the comment %s, and it is drawn on the row of %s", view.Ref, entity.ID, view.ID)
		}
	}
}

// TestEveryChecklistItemCarriesAReferenceThatResolves asserts dinah-454 AC-4:
// every checklist row carries an address, including a row whose kind is none
// of the three the format declares.
//
// The third item is written by hand because no verb files an undeclared kind,
// which is the only way the case can exist at all. Its reference is the
// uncollected one, since the resolver descends the collection unnarrowed for a
// segment that names no checklist kind, and the two declared kinds carry the
// word their kind is addressed by.
func TestEveryChecklistItemCarriesAReferenceThatResolves(t *testing.T) {
	h := newHarness(t)
	card := h.add("a card carrying three kinds of item")
	h.file(card, "open_question", "the question")
	h.file(card, "acceptance_criterion", "the criterion")
	h.item(card, "c00000000003", "kind: risk\nordinal: 3\n", "a risk")

	detail, _, err := h.library.Show(&Request{Verb: "show", Card: card})
	if err != nil {
		t.Fatalf("show %s: %v", card, err)
	}
	if len(detail.Checklist) != 3 {
		t.Fatalf("the card carries %d items, wanted the three the fixture wrote", len(detail.Checklist))
	}
	byKind := map[string]ItemView{}
	for _, view := range detail.Checklist {
		byKind[view.Kind] = view
		if view.Ref == "" {
			t.Errorf("the item %s of kind %q carries no reference, so nothing on the row tells a reader how to name it", view.ID, view.Kind)
			continue
		}
		entity, err := h.library.Bench.ResolveEntity(view.Ref)
		if err != nil {
			t.Errorf("the item %s is addressed %q, which resolves to nothing: %v", view.ID, view.Ref, err)
			continue
		}
		if entity.ID != view.ID {
			t.Errorf("%q resolves to the item %s, and it is drawn on the row of %s", view.Ref, entity.ID, view.ID)
		}
	}
	if got, want := byKind["risk"].Ref, card+"/checklist/3"; got != want {
		t.Errorf("the risk item is addressed %q, wanted the collection spelling %q, since the format declares no word for that kind", got, want)
	}
	if got, want := byKind["acceptance_criterion"].Ref, card+"/criteria/1"; got != want {
		t.Errorf("the criterion is addressed %q, wanted the kind-narrowed %q", got, want)
	}
}
