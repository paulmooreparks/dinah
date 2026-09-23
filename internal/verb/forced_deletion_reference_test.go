package verb

import (
	"os"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// TestTheForcedDeletionStillHandsReopenAReferenceThatResolves is
// dinah-472/criteria/75, and it guards a sentence an earlier draft of this
// card's contract wrongly told the implementer was obsolete.
//
// The sentence says the item reference handed to Reopen is composed rather
// than taken from the item's own identifier, because Reopen resolves what it
// is handed and a bare identifier resolves to nothing. dinah-472 made a bare
// identifier the stored form of an item's answer, which is a member selector
// inside one collection, and left what a whole reference may be exactly as it
// was. The two are different questions, and an implementer who read the
// sentence as obsolete would replace the composed reference with `item.ID` and
// break the forced-deletion path, which is where the operator-owned check
// lives.
//
// Both halves are asserted. The source still carries the reasoning, so nobody
// deletes it as stale a second time, and the forced deletion still works,
// which is the behaviour the reasoning is about.
func TestTheForcedDeletionStillHandsReopenAReferenceThatResolves(t *testing.T) {
	source, err := os.ReadFile("beyond.go")
	if err != nil {
		t.Fatalf("read beyond.go: %v", err)
	}
	const reasoning = "// The reference is composed rather than taken from the item's own\n" +
		"\t// identifier, because Reopen resolves what it is handed and a bare\n" +
		"\t// identifier resolves to nothing."
	if !strings.Contains(string(source), reasoning) {
		t.Error("admitCommentDeletion no longer carries the reasoning for composing the reference it hands Reopen")
	}
	if !strings.Contains(string(source), "named, err := l.itemCanonicalRef(entity.Card, item.ID)") {
		t.Error("admitCommentDeletion no longer composes the item reference it hands Reopen")
	}

	// And the behaviour, because a comment nothing exercises is a claim
	// rather than a guard. A bare identifier does not resolve as a whole
	// reference, which is the fact the reasoning rests on.
	h := newHarness(t)
	card := h.add("a card carrying a settled question")
	item := h.file(card, "open_question", "does the deadline move?")
	settled := h.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: item, Text: "the operator ruled"})
	if settled.Outcome != contract.OutcomeOK {
		t.Fatalf("resolve: %s %s", settled.Outcome, settled.Refusal)
	}
	h.reopen()
	entity, err := h.library.Bench.ResolveEntity(item)
	if err != nil {
		t.Fatalf("resolve %s: %v", item, err)
	}
	stored, err := bench.LoadItem(entity.Dir)
	if err != nil {
		t.Fatalf("load %s: %v", item, err)
	}
	if !bench.IsID(stored.Resolution) {
		t.Fatalf("the item designates %q, so this case is not exercising the stored form it is about", stored.Resolution)
	}
	if _, err := h.library.Bench.ResolveEntity(stored.ID); err == nil {
		t.Error("an item's bare identifier resolves as a whole reference, so the reasoning this case guards has stopped being true")
	}

	dir, found := h.library.Bench.DesignatedCommentDir(stored)
	if !found {
		t.Fatalf("the item's answer names no comment of it")
	}
	forced := h.library.Delete(&Request{
		Verb: "delete", Actor: "alka", Ref: item + "/" + bench.CommentsDir + "/1", Confirm: true, Force: true,
	})
	if forced.Outcome != contract.OutcomeOK {
		t.Fatalf("the forced delete: %s %s", forced.Outcome, forced.Refusal)
	}
	h.reopen()
	if bench.Exists(dir) {
		t.Error("the comment survived the forced delete")
	}
	fm, _ := h.itemAnchor(item)
	if got := fm.Value(bench.ItemStateField); got != bench.ItemPending {
		t.Errorf("the forced delete left the item at %q, wanted %q, so the reopen it composes did not land", got, bench.ItemPending)
	}
}
