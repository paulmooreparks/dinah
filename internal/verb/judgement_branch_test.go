package verb

import (
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
)

// TestGroupingTakesItsRosterAndOrderFromTheDeclaration is the derivation check
// this card rests on, and it is a behavioural one rather than a scan of the
// source.
//
// The declaration it hands in carries four kinds, one of them invented, in an
// order that is not the declaration the tool ships. An implementation holding
// a roster of its own cannot answer four branches, and one holding an order of
// its own cannot answer them in this order. That is the wrong implementation
// this case is armed against: replace the argument with a literal table of the
// three shipped kinds and both assertions below go red.
func TestGroupingTakesItsRosterAndOrderFromTheDeclaration(t *testing.T) {
	declared := []bench.ChecklistKind{
		{Kind: "decision", Word: "decisions"},
		{Kind: "risk", Word: "risks"},
		{Kind: "open_question", Word: "questions"},
		{Kind: "acceptance_criterion", Word: "criteria"},
	}
	items := []kindedNode{
		{Node: TreeNode{Kind: bench.KindItem, Ref: "fx-1/criteria/1"}, Kind: "acceptance_criterion"},
		{Node: TreeNode{Kind: bench.KindItem, Ref: "fx-1/risks/1"}, Kind: "risk"},
		{Node: TreeNode{Kind: bench.KindItem, Ref: "fx-1/questions/1"}, Kind: "open_question"},
		{Node: TreeNode{Kind: bench.KindItem, Ref: "fx-1/decisions/1"}, Kind: "decision"},
	}

	branches := groupChecklist(declared, items, "fx-1")
	if len(branches) != 4 {
		t.Fatalf("the grouping drew %d branches from a declaration of 4", len(branches))
	}
	for i, want := range []string{"fx-1/decisions", "fx-1/risks", "fx-1/questions", "fx-1/criteria"} {
		if branches[i].Ref != want {
			t.Errorf("branch %d is %s and the declaration puts %s there", i, branches[i].Ref, want)
		}
		if branches[i].Kind != KindCollection {
			t.Errorf("%s is drawn as %s rather than as a collection", branches[i].Ref, branches[i].Kind)
		}
		if branches[i].MemberKind != bench.KindItem {
			t.Errorf("%s holds member kind %q, want item", branches[i].Ref, branches[i].MemberKind)
		}
		if branches[i].Narrow != declared[i].Kind {
			t.Errorf("%s narrows by %q, want %q", branches[i].Ref, branches[i].Narrow, declared[i].Kind)
		}
	}
}

// TestGroupingOpensNothing asserts that the grouping reads no anchor, which is
// half of the promise that one projection reads each item once. Every node it
// is handed names a directory that is not there, so an implementation that
// opened one to ask its kind would answer no branches at all.
func TestGroupingOpensNothing(t *testing.T) {
	declared := bench.ChecklistKinds()
	items := []kindedNode{
		{Node: TreeNode{Kind: bench.KindItem, Ref: "fx-1/questions/1", ID: "nowhere00001"}, Kind: "open_question"},
	}
	branches := groupChecklist(declared, items, "fx-1")
	if len(branches) != 1 || branches[0].Ref != "fx-1/questions" {
		t.Fatalf("the grouping drew %d branches from one question, want the questions branch alone", len(branches))
	}
}

// TestOneProjectionReadsEachItemOnce counts the reads the projection makes,
// which is what the read bound is about: the grouping must not walk the
// checklist a second time to ask each item what it is.
//
// The wrong implementation is the one this card was first written with, where
// the grouping re-opened every anchor the walk had already opened. Under it
// every count below is two rather than one.
func TestOneProjectionReadsEachItemOnce(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card whose items are counted")
	card := h.card(ref)
	writeItemOfKind(t, card.Dir, "open_question", "the first question", 1)
	writeItemOfKind(t, card.Dir, "acceptance_criterion", "the first criterion", 2)
	writeItemOfKind(t, card.Dir, "decision", "the first decision", 3)

	reads := map[string]int{}
	real := itemKindAt
	itemKindAt = func(dir string) string {
		reads[filepath.Base(dir)]++
		return real(dir)
	}
	defer func() { itemKindAt = real }()

	built := contentsOf(t, h, ref, LevelAll)
	if len(reads) != 3 {
		t.Fatalf("the projection read %d items and the card carries 3", len(reads))
	}
	for id, count := range reads {
		if count != 1 {
			t.Errorf("the projection read item %s %d times, and one projection reads each item once", id, count)
		}
	}
	// The branches were still drawn, so the count above is the count of a
	// projection that did the work rather than of one that skipped it.
	branches := 0
	walkTree(built.Root, func(node TreeNode) {
		if node.Kind == KindCollection {
			branches++
		}
	})
	if branches != 3 {
		t.Fatalf("the projection drew %d branches while reading each item once, want 3", branches)
	}
}

// TestADamagedOrUndeclaredItemStaysVisibleExactlyOnce holds the rule that the
// projection drops nothing. An item whose anchor will not open has no kind to
// group it by, and an item carrying a kind outside the declaration is not the
// projection's to invent a branch for. Both stay where they were, as direct
// children with the reference the walk already gave them.
func TestADamagedOrUndeclaredItemStaysVisibleExactlyOnce(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card with a damaged item and an undeclared one")
	card := h.card(ref)
	writeItemOfKind(t, card.Dir, "open_question", "a readable question", 1)
	writeItemOfKind(t, card.Dir, "risk", "an undeclared kind", 2)
	writeDamagedItem(t, card.Dir, 3)

	built := contentsOf(t, h, ref, LevelAll)
	seen := map[string]int{}
	walkTree(built.Root, func(node TreeNode) {
		if node.Kind == bench.KindItem {
			seen[node.Ref]++
		}
	})
	if len(seen) != 3 {
		t.Fatalf("the walk drew %d item references and the card carries 3: %v", len(seen), seen)
	}
	for reference, count := range seen {
		if count != 1 {
			t.Errorf("%s is drawn %d times and every item is drawn once", reference, count)
		}
	}
	// The two the projection could not place keep the checklist reference,
	// which is the fallback the walk composed before this card and still
	// resolves. Neither is given a narrowed reference it has no kind for.
	fallbacks := 0
	for reference := range seen {
		if strings.Contains(reference, "/checklist/") {
			fallbacks++
		}
	}
	if fallbacks != 2 {
		t.Errorf("%d items kept the checklist fallback reference and 2 could not be placed: %v", fallbacks, seen)
	}
	// They follow the branches rather than standing among them.
	var tail []TreeNode
	branchesSeen := false
	for _, child := range built.Root.Children {
		if child.Kind == KindCollection {
			branchesSeen = true
			if len(tail) > 0 {
				t.Errorf("%s stands after an unplaced item, and the unplaced ones follow every branch", child.Ref)
			}
			continue
		}
		if child.Kind == bench.KindItem {
			tail = append(tail, child)
		}
	}
	if !branchesSeen {
		t.Fatal("the card drew no branch, so this case proves nothing about where an unplaced item stands")
	}
	if len(tail) != 2 {
		t.Fatalf("the card draws %d direct item children and 2 could not be placed", len(tail))
	}

	// The reference each unplaced item draws is one that opens that item and
	// not its neighbour, which is the whole of what a fallback reference owes
	// a reader. Positions are not asserted: the order the checklist sorts in
	// is the collection's own business and an item with no readable ordinal
	// takes whatever place that sort gives it.
	for _, node := range tail {
		path, err := h.library.Bench.ResolvePath(node.Ref)
		if err != nil {
			t.Errorf("%s does not resolve: %v", node.Ref, err)
			continue
		}
		if reached := filepath.Base(filepath.Dir(path)); reached != node.ID {
			t.Errorf("%s names the item %s and opens %s instead", node.Ref, node.ID, reached)
		}
	}
}

// TestADepthCutBeforeAChecklistReportsItsItems holds the rule that a branch is
// no entity. A cut made above a card's members reports the members held back,
// which is what it reported before this card, rather than the branches that
// would have stood in front of them.
func TestADepthCutBeforeAChecklistReportsItsItems(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card cut off above its checklist")
	card := h.card(ref)
	writeItemOfKind(t, card.Dir, "open_question", "the first question", 1)
	writeItemOfKind(t, card.Dir, "open_question", "the second question", 2)
	writeItemOfKind(t, card.Dir, "decision", "the first decision", 3)

	built := contentsOf(t, h, ref, LevelRoot)
	if built.Root.Children != nil {
		t.Fatalf("the card drew %d children at its own level", len(built.Root.Children))
	}
	if built.Root.Hidden == nil {
		t.Fatal("the card held its members back and reported nothing")
	}
	if built.Root.Hidden.Children != 3 {
		t.Errorf("the card reports %d children held back, and it holds 3 items and no branches",
			built.Root.Hidden.Children)
	}
}

// writeItemOfKind writes one checklist item of a named kind by hand, which
// fixes the identifier and the ordinal these cases order their assertions on.
func writeItemOfKind(t *testing.T, cardDir, kind, text string, ordinal int) {
	t.Helper()
	id := "d0000000000" + string(rune('0'+ordinal))
	fm := bench.NewFrontmatter()
	fm.Set("kind", kind)
	fm.Set("title", text)
	fm.Set(bench.OrdinalField, string(rune('0'+ordinal)))
	path := filepath.Join(cardDir, bench.ChecklistDir, id, bench.ItemAnchor)
	if err := bench.WriteText(path, fm.Render("")); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// writeDamagedItem writes an item whose anchor will not parse as one, which is
// the member the projection can read no kind from.
func writeDamagedItem(t *testing.T, cardDir string, ordinal int) {
	t.Helper()
	id := "d0000000000" + string(rune('0'+ordinal))
	path := filepath.Join(cardDir, bench.ChecklistDir, id, bench.ItemAnchor)
	if err := bench.WriteText(path, "this anchor carries no front matter and no kind\n"); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
