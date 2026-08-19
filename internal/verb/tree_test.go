package verb

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// treeOf builds a grouped tree and fails the test unless it was built.
func treeOf(t *testing.T, h *harness, query string, chain []string, level string) *Tree {
	t.Helper()
	built, err := h.library.Tree(&Request{Verb: "tree", Query: query}, chain, level)
	if err != nil {
		t.Fatalf("tree %q along %v at %s: %v", query, chain, level, err)
	}
	return built
}

// contentsOf walks the containment grammar and fails the test unless it walked.
func contentsOf(t *testing.T, h *harness, ref, level string) *Tree {
	t.Helper()
	built, err := h.library.Contents(&Request{Verb: "contents", Ref: ref}, level)
	if err != nil {
		t.Fatalf("contents %q at %s: %v", ref, level, err)
	}
	return built
}

// groupAt returns the child of a node grouped at a value, and fails the test
// when the node draws no such group.
func groupAt(t *testing.T, node TreeNode, value string) TreeNode {
	t.Helper()
	for _, child := range node.Children {
		if child.Kind == NodeGroup && child.Value == value {
			return child
		}
	}
	t.Fatalf("no group at %q among %v", value, groupValues(node))
	return TreeNode{}
}

// groupValues names every group a node draws, for a failure message.
func groupValues(node TreeNode) []string {
	var values []string
	for _, child := range node.Children {
		values = append(values, child.Value)
	}
	return values
}

// leafRefs collects the reference of every card node in a tree.
func leafRefs(node TreeNode) []string {
	var refs []string
	if node.Kind == bench.KindCard {
		refs = append(refs, node.Ref)
	}
	for _, child := range node.Children {
		refs = append(refs, leafRefs(child)...)
	}
	return refs
}

// walkTree calls visit on every node of a tree, root included.
func walkTree(node TreeNode, visit func(TreeNode)) {
	visit(node)
	for _, child := range node.Children {
		walkTree(child, visit)
	}
}

// joinWorkstreams puts a card in the workstreams named, writing the membership
// the way the format stores it. Nothing in the tool offers this yet, since no
// verb creates a workstream.
func joinWorkstreams(t *testing.T, h *harness, ref string, names ...string) {
	t.Helper()
	card := h.card(ref)
	card.Workstreams = names
	if err := card.Save(); err != nil {
		t.Fatalf("save %s: %v", ref, err)
	}
	h.reopen()
}

// TestAGroupCountsItsOwnCardsRatherThanItsChildren asserts that a node's count
// is the size of its own subject set. Two of the six cards below belong to two
// workstreams each, so the three groups count eight cards between them while
// the workbench holds six, and an implementation adding its children up
// reports eight at the root and passes every other criterion here.
func TestAGroupCountsItsOwnCardsRatherThanItsChildren(t *testing.T) {
	h := newHarness(t)
	both := []string{h.add("both one"), h.add("both two")}
	for _, ref := range both {
		joinWorkstreams(t, h, ref, "alpha", "beta")
	}
	joinWorkstreams(t, h, h.add("alpha only"), "alpha")
	joinWorkstreams(t, h, h.add("beta only"), "beta")
	h.add("unassigned one")
	h.add("unassigned two")

	built := treeOf(t, h, "", []string{FieldWorkstream}, LevelCards)
	if built.Root.Count != 6 {
		t.Errorf("the root counts %d cards and the workbench holds 6", built.Root.Count)
	}
	wanted := map[string]int{"alpha": 3, "beta": 3, "": 2}
	if len(built.Root.Children) != len(wanted) {
		t.Fatalf("the root draws %d groups, want %d: %v", len(built.Root.Children), len(wanted), groupValues(built.Root))
	}
	total := 0
	for value, count := range wanted {
		group := groupAt(t, built.Root, value)
		if group.Count != count {
			t.Errorf("the group at %q counts %d and holds %d cards", value, group.Count, count)
		}
		total += group.Count
	}
	if total != 8 {
		t.Errorf("the three groups count %d cards between them, and the fixture makes that 8", total)
	}
}

// TestTheNoValueGroupComesLast asserts the order of an open-valued axis: a
// group per value some card carries, sorted by the value's bytes, and the
// group holding the cards that carry no value last.
func TestTheNoValueGroupComesLast(t *testing.T) {
	h := newHarness(t)
	first := h.add("held by one")
	second := h.add("held by another")
	for range 4 {
		h.add("unheld")
	}
	h.mustDo(&Request{Verb: Claim, Actor: "zoya", Card: first, Holder: "zoya"})
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: second, Holder: "alka"})

	built := treeOf(t, h, "", []string{FieldHolder}, LevelCards)
	var drawn []string
	for _, child := range built.Root.Children {
		drawn = append(drawn, child.Value)
	}
	want := []string{"alka", "zoya", ""}
	if strings.Join(drawn, "|") != strings.Join(want, "|") {
		t.Errorf("the holders draw as %v and the axis orders them %v", drawn, want)
	}
}

// TestDepthReportsAtTheBoundaryAndNowhereElse asserts that a node reports the
// children the depth cut off directly beneath it and says nothing about a cut
// further down, and that its count is unchanged by the cut.
func TestDepthReportsAtTheBoundaryAndNowhereElse(t *testing.T) {
	h := newHarness(t)
	for range 4 {
		h.add("a card of the intake")
	}
	built := treeOf(t, h, "", nil, LevelGroups)

	state := groupAt(t, built.Root, "intake")
	if state.Count != 4 {
		t.Errorf("the state counts %d cards and holds 4", state.Count)
	}
	if state.Hidden != nil {
		t.Errorf("the state reports %v and every child it has is drawn", state.Hidden)
	}
	ready := groupAt(t, state, contract.SubstateReady)
	if ready.Hidden == nil {
		t.Fatalf("the ready group reports nothing and the depth cut its cards off")
	}
	if got := strings.Join(ready.Hidden.Reason, ","); got != ReasonDepth {
		t.Errorf("the ready group reports the reasons %q, want %q", got, ReasonDepth)
	}
	if ready.Count != 4 || ready.Hidden.Children != 4 || ready.Hidden.Subjects != 4 {
		t.Errorf("the ready group reports count %d, %d children and %d subjects, want 4, 4 and 4",
			ready.Count, ready.Hidden.Children, ready.Hidden.Subjects)
	}
	walkTree(built.Root, func(node TreeNode) {
		if node.Kind == bench.KindCard {
			t.Errorf("a card node was drawn at the groups level: %s", node.Ref)
		}
	})
}

// TestAContainedNodeCountsWhatItContainsAndNeverItself asserts the containment
// counting rule and the identity that follows from it, by walking the tree
// rather than by naming the numbers a second time.
func TestAContainedNodeCountsWhatItContainsAndNeverItself(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card with things below it")
	card := h.card(ref)
	h.comment(ref, "the first note")
	h.comment(ref, "the second note")
	writeItem(t, card.Dir, "AC-1", 1)
	writeItem(t, card.Dir, "AC-2", 2)
	h.attach(ref, "notes.txt", "some bytes")

	built := contentsOf(t, h, ref, LevelAll)
	if built.Root.Count != 5 {
		t.Errorf("the card counts %d entities and carries 5", built.Root.Count)
	}
	if len(built.Root.Children) != 5 {
		t.Fatalf("the card draws %d children and carries 5", len(built.Root.Children))
	}
	for _, child := range built.Root.Children {
		if child.Count != 0 {
			t.Errorf("%s counts %d and contains nothing", child.Ref, child.Count)
		}
	}
	walkTree(built.Root, func(node TreeNode) {
		total := len(node.Children)
		for _, child := range node.Children {
			total += child.Count
		}
		if node.Count != total {
			t.Errorf("%s counts %d and its children and their counts add to %d", node.Kind, node.Count, total)
		}
	})
}

// writeItem writes a checklist item by hand, since no verb files one yet.
func writeItem(t *testing.T, cardDir, text string, ordinal int) {
	t.Helper()
	id := "c0000000000" + string(rune('0'+ordinal))
	fm := bench.NewFrontmatter()
	fm.Set("kind", "acceptance_criterion")
	fm.Set("title", text)
	fm.Set(bench.OrdinalField, string(rune('0'+ordinal)))
	path := filepath.Join(cardDir, bench.ChecklistDir, id, bench.ItemAnchor)
	if err := bench.WriteText(path, fm.Render("")); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestAnOpenValuedGroupSurvivesAFilterThatEmptiesIt asserts that a group whose
// every card the filter removed is still drawn, with the count it has left and
// an account of what went. A closed-enum axis cannot discriminate the two
// readings, so the axis here is open-valued.
func TestAnOpenValuedGroupSurvivesAFilterThatEmptiesIt(t *testing.T) {
	h := newHarness(t)
	held := h.add("the card the holder is working")
	h.add("a card nobody holds")
	h.mustDo(&Request{Verb: Claim, Actor: "zoya", Card: held, Holder: "zoya"})

	built := treeOf(t, h, "substate:ready", []string{FieldHolder}, LevelCards)
	group := groupAt(t, built.Root, "zoya")
	if group.Count != 0 {
		t.Errorf("the holder's group counts %d and the filter left it nothing", group.Count)
	}
	if group.Hidden == nil || group.Hidden.Filtered != 1 {
		t.Fatalf("the holder's group reports %v and the filter removed one card from it", group.Hidden)
	}
	if got := strings.Join(group.Hidden.Reason, ","); got != ReasonFilter {
		t.Errorf("the holder's group reports the reasons %q, want %q", got, ReasonFilter)
	}
}

// TestBothReasonsAreReportedSeparately asserts that a node holding something
// back for both reasons names them in the fixed order and reports two numbers
// that do not agree with each other, which is the fixture's whole point.
func TestBothReasonsAreReportedSeparately(t *testing.T) {
	h := newHarness(t)
	for range 3 {
		h.add("a ready card of the intake")
	}
	held := h.add("the card the holder is working")
	h.mustDo(&Request{Verb: Claim, Actor: "zoya", Card: held, Holder: "zoya"})

	built := treeOf(t, h, "substate:ready", nil, LevelGroups)
	state := groupAt(t, built.Root, "intake")
	if state.Hidden == nil {
		t.Fatalf("the state reports nothing and the filter removed a card below it")
	}
	if got := strings.Join(state.Hidden.Reason, ","); got != ReasonFilter {
		t.Errorf("the state reports the reasons %q, and its own children are all drawn, want %q", got, ReasonFilter)
	}
	ready := groupAt(t, state, contract.SubstateReady)
	if ready.Hidden == nil {
		t.Fatalf("the ready group reports nothing and the depth cut its cards off")
	}
	if got := strings.Join(ready.Hidden.Reason, ","); got != ReasonDepth {
		t.Errorf("the ready group reports the reasons %q, want %q", got, ReasonDepth)
	}
	active := groupAt(t, state, contract.SubstateActive)
	if active.Hidden == nil {
		t.Fatalf("the active group reports nothing and the filter removed its one card")
	}
	if got := strings.Join(active.Hidden.Reason, ","); got != ReasonFilter {
		t.Errorf("the active group reports the reasons %q, want %q", got, ReasonFilter)
	}
	if active.Hidden.Children != 0 || active.Hidden.Filtered != 1 {
		t.Errorf("the active group reports %d children and %d filtered, want 0 and 1",
			active.Hidden.Children, active.Hidden.Filtered)
	}
}

// TestReasonsAreNamedInTheFixedOrder asserts the order of the pair on a node
// carrying both, with the two numbers asserted independently and a fixture
// chosen so they differ.
func TestReasonsAreNamedInTheFixedOrder(t *testing.T) {
	h := newHarness(t)
	for range 3 {
		h.add("a ready card")
	}
	held := h.add("the card the holder is working")
	h.mustDo(&Request{Verb: Claim, Actor: "zoya", Card: held, Holder: "zoya"})

	built := treeOf(t, h, "substate:ready", []string{FieldState}, LevelGroups)
	state := groupAt(t, built.Root, "intake")
	if state.Hidden == nil {
		t.Fatalf("the state reports nothing and it is holding two kinds of thing back")
	}
	want := []string{ReasonDepth, ReasonFilter}
	if got := strings.Join(state.Hidden.Reason, ","); got != strings.Join(want, ",") {
		t.Errorf("the state names the reasons %q and the order is fixed at %q", got, strings.Join(want, ","))
	}
	if state.Hidden.Children != 3 {
		t.Errorf("the state reports %d children cut off and the depth cut off 3", state.Hidden.Children)
	}
	if state.Hidden.Subjects != 3 {
		t.Errorf("the state reports %d surviving subjects below and 3 survived", state.Hidden.Subjects)
	}
	if state.Hidden.Filtered != 1 {
		t.Errorf("the state reports %d filtered out and the filter removed 1", state.Hidden.Filtered)
	}
}

// TestAFullyExpandedTreeHidesNothingAnywhere asserts the quiet side of the
// elision rule by walking every node of three trees rather than by sampling
// one. Without it an implementation emitting an account at every node, or at
// every boundary whether or not anything was cut, passes every other criterion
// here.
func TestAFullyExpandedTreeHidesNothingAnywhere(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card carrying a comment")
	h.comment(ref, "a note with nothing attached to it")
	h.add("a card carrying nothing")

	grouped := treeOf(t, h, "", nil, LevelCards)
	assertNothingIsHidden(t, "the grouped tree", grouped.Root)
	if empty := groupAt(t, groupAt(t, grouped.Root, "intake"), contract.SubstateBlocked); empty.Count != 0 {
		t.Errorf("the blocked group counts %d and the fixture blocks nothing", empty.Count)
	}
	assertNothingIsHidden(t, "the whole containment walk", contentsOf(t, h, "workbench", LevelAll).Root)
	// A bounded depth that reaches its boundary and finds nothing to cut is
	// the third case, and it is the one an implementation reporting at every
	// boundary gets wrong.
	assertNothingIsHidden(t, "the card at the default depth", contentsOf(t, h, ref, LevelEntities).Root)
}

// assertNothingIsHidden fails on any node of a tree carrying an account of
// something it is not showing.
func assertNothingIsHidden(t *testing.T, what string, root TreeNode) {
	t.Helper()
	walkTree(root, func(node TreeNode) {
		if node.Hidden != nil {
			t.Errorf("%s: %s %s reports %v and hides nothing", what, node.Kind, node.Ref+node.Value, node.Hidden)
		}
	})
}

// TestTheTreeAndTheQuerySelectTheSameCards asserts that one query string hands
// both commands the same cards.
//
// The fixture carries a card that satisfies the two act-plane terms through two
// different acts, which the single-witness rule excludes, so a tree
// reimplementing the filter term by term rather than calling the query's own
// matcher draws that card and fails. Evaluating the filter once over the card
// set and evaluating it again at every node select the same cards, since a
// filter is a predicate on a card, so this test does not discriminate those two
// and does not claim to.
func TestTheTreeAndTheQuerySelectTheSameCards(t *testing.T) {
	h := newHarness(t)
	split := h.add("moved into doing by one owner and claimed later by another")
	h.mustDo(&Request{Verb: Claim, Actor: "zoya", Card: split, Holder: "zoya"})
	h.mustDo(&Request{Verb: Move, Actor: "zoya", Card: split, State: doing})
	h.mustDo(&Request{Verb: Move, Actor: "zoya", Card: split, State: review})
	h.mustDo(&Request{Verb: Release, Actor: "zoya", Card: split})
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: split, Holder: "alka"})
	h.add("a card nobody has moved")

	witnessed := h.add("moved into doing by the same owner who claimed it")
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: witnessed, Holder: "alka"})
	h.mustDo(&Request{Verb: Move, Actor: "alka", Card: witnessed, State: doing})

	const query = "entered:doing actor:alka"
	matches, err := h.library.Query(&Request{Verb: "query", Query: query})
	if err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	var queried []string
	for _, card := range matches.Cards {
		queried = append(queried, card.Ref)
	}
	drawn := leafRefs(treeOf(t, h, query, nil, LevelCards).Root)
	sort.Strings(queried)
	sort.Strings(drawn)
	if strings.Join(queried, ",") != strings.Join(drawn, ",") {
		t.Errorf("the query selects %v and the tree draws %v", queried, drawn)
	}
	if len(queried) != 1 || queried[0] != witnessed {
		t.Fatalf("the query selects %v, and the single-witness rule admits only %s", queried, witnessed)
	}
	if contains(drawn, split) {
		t.Errorf("the tree draws %s, whose two terms are satisfied by two different acts", split)
	}
}

// TestTheDispositionTablePairsWithTheQueryFields asserts that the axis
// vocabulary and the query vocabulary close over each other in both
// directions, so a field added or renamed on that side fails the build here
// rather than becoming silently ungroupable.
func TestTheDispositionTablePairsWithTheQueryFields(t *testing.T) {
	declared := map[string]int{}
	for _, row := range AxisDispositions {
		declared[row.Field]++
		if row.Disposition != DispositionAxis && row.Disposition != DispositionRefused {
			t.Errorf("%s carries the disposition %q, which is neither %q nor %q", row.Field, row.Disposition, DispositionAxis, DispositionRefused)
		}
		if row.Enumeration != EnumerationClosed && row.Enumeration != EnumerationOpen {
			t.Errorf("%s carries the enumeration %q, which is neither %q nor %q", row.Field, row.Enumeration, EnumerationClosed, EnumerationOpen)
		}
	}
	for _, field := range QueryFields {
		if declared[field] == 0 {
			t.Errorf("the query language declares the field %s and the disposition table has no row for it, so it is silently ungroupable", field)
		}
		if declared[field] > 1 {
			t.Errorf("the disposition table carries %d rows for %s, and a field has one", declared[field], field)
		}
	}
	for _, row := range AxisDispositions {
		if !contains(QueryFields, row.Field) {
			t.Errorf("the disposition table names %s and the query language declares no such field", row.Field)
		}
	}
	closed := 0
	for _, row := range AxisDispositions {
		if row.Enumeration == EnumerationClosed {
			closed++
		}
	}
	if closed != 2 {
		t.Errorf("the table reads %s on %d rows, and %s and %s are the only two axes that enumerate", EnumerationClosed, closed, FieldState, FieldSubstate)
	}
	if len(GroupAxes()) != 9 {
		t.Errorf("the table admits %d axes, and nine of the ten fields group", len(GroupAxes()))
	}
}

// TestAFilteredTreeAccountsForEveryCardItRemoved asserts that the counts and
// the account add back up to what the workbench holds, at the root and at a
// group the filter emptied.
func TestAFilteredTreeAccountsForEveryCardItRemoved(t *testing.T) {
	h := newHarness(t)
	first := h.add("the first active card")
	second := h.add("the second active card")
	h.add("a ready card of the intake")
	for _, ref := range []string{first, second} {
		h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: ref, Holder: "alka"})
		h.mustDo(&Request{Verb: Move, Actor: "alka", Card: ref, State: review})
	}

	built := treeOf(t, h, "substate:ready", nil, LevelCards)
	if built.Root.Count != 1 || built.Root.Hidden == nil || built.Root.Hidden.Filtered != 2 {
		t.Fatalf("the root counts %d and reports %v, and the workbench holds three cards of which one matches", built.Root.Count, built.Root.Hidden)
	}
	state := groupAt(t, built.Root, "review")
	if state.Count != 0 {
		t.Errorf("the Review group counts %d and the filter left it nothing", state.Count)
	}
	if state.Hidden == nil || state.Hidden.Filtered != 2 {
		t.Fatalf("the Review group reports %v and the filter removed two cards from it", state.Hidden)
	}
	ready := groupAt(t, state, contract.SubstateReady)
	if ready.Count != 0 {
		t.Errorf("the ready group inside Review counts %d and holds nothing", ready.Count)
	}
	if ready.Hidden != nil {
		t.Errorf("the ready group inside Review reports %v and it hides nothing at all", ready.Hidden)
	}
}

// TestTheDefaultChainDrawsTheStatusTree asserts that a bare tree nests state
// over substate over cards, enumerates both closed axes completely and in
// their declared order, and orders the cards of a group the way a listing of
// that state orders them.
func TestTheDefaultChainDrawsTheStatusTree(t *testing.T) {
	h := newHarness(t)
	first := h.add("the first card filed")
	second := h.add("the second card filed")
	h.renumber(h.card(second).ID, 1)
	h.renumber(h.card(first).ID, 2)
	h.reopen()

	built := treeOf(t, h, "", nil, LevelCards)
	var states []string
	for _, child := range built.Root.Children {
		states = append(states, child.Value)
	}
	want := []string{"intake", "doing", "review", "finished", aftercareSlug}
	if strings.Join(states, ",") != strings.Join(want, ",") {
		t.Errorf("the states draw as %v and the flow declares them %v", states, want)
	}
	intakeGroup := groupAt(t, built.Root, "intake")
	var substates []string
	for _, child := range intakeGroup.Children {
		substates = append(substates, child.Value)
	}
	wantSubstates := []string{contract.SubstateReady, contract.SubstateActive, contract.SubstateBlocked}
	if strings.Join(substates, ",") != strings.Join(wantSubstates, ",") {
		t.Errorf("the substates draw as %v and the order is fixed at %v", substates, wantSubstates)
	}
	listing, err := h.library.List(&Request{Verb: "ls", State: "intake"})
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	var listed []string
	for _, card := range listing.Cards {
		listed = append(listed, card.Ref)
	}
	drawn := leafRefs(intakeGroup)
	if strings.Join(listed, ",") != strings.Join(drawn, ",") {
		t.Errorf("the tree draws the cards as %v and ls orders them %v", drawn, listed)
	}
}

// TestAnActPlaneAxisReadsTheWholeActRecord asserts that a card two owners have
// acted on is drawn under both, and that the filter's own witness does not
// narrow where grouping puts a card.
func TestAnActPlaneAxisReadsTheWholeActRecord(t *testing.T) {
	h := newHarness(t)
	shared := h.add("a card two owners have acted on")
	h.mustDo(&Request{Verb: Claim, Actor: "zoya", Card: shared, Holder: "zoya"})
	h.mustDo(&Request{Verb: Release, Actor: "zoya", Card: shared})
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: shared, Holder: "alka"})

	built := treeOf(t, h, "", []string{FieldActor}, LevelCards)
	for _, actor := range []string{"alka", "zoya"} {
		if refs := leafRefs(groupAt(t, built.Root, actor)); !contains(refs, shared) {
			t.Errorf("the group at %q draws %v and the card was acted on by that owner", actor, refs)
		}
	}
	total := 0
	for _, child := range built.Root.Children {
		total += child.Count
	}
	if total <= built.Root.Count {
		t.Errorf("the actor groups count %d between them and the root counts %d, so nothing fanned out", total, built.Root.Count)
	}

	// The filter selects on one witnessing act and the grouping reads them
	// all, so the selected card is still drawn under the owner the filter
	// never named.
	filtered := treeOf(t, h, "actor:alka", []string{FieldActor}, LevelCards)
	if refs := leafRefs(groupAt(t, filtered.Root, "zoya")); !contains(refs, shared) {
		t.Errorf("the filtered tree draws %v under zoya, and grouping reads the whole act record", refs)
	}
}

// TestOnlyTheTwoClosedAxesEnumerateTheirMembers asserts that a state nobody
// has entered draws a group under state and none under entered, which is what
// separates a closed axis from a journal-shaped one.
func TestOnlyTheTwoClosedAxesEnumerateTheirMembers(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card that has only ever been in two states")
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: ref, Holder: "alka"})
	h.mustDo(&Request{Verb: Move, Actor: "alka", Card: ref, State: doing})

	byState := treeOf(t, h, "", []string{FieldState}, LevelCards)
	if len(byState.Root.Children) != 5 {
		t.Errorf("the state axis draws %d groups and the workbench declares five states", len(byState.Root.Children))
	}
	byEntered := treeOf(t, h, "", []string{FieldEntered}, LevelCards)
	var entered []string
	for _, child := range byEntered.Root.Children {
		entered = append(entered, child.Value)
	}
	sort.Strings(entered)
	if strings.Join(entered, ",") != "doing,intake" {
		t.Errorf("the entered axis draws groups for %v, and the card has entered only two states", entered)
	}
	blocks := treeOf(t, h, "", []string{FieldBlockKind}, LevelCards)
	for _, child := range blocks.Root.Children {
		if child.Value != "" {
			t.Errorf("the block_kind axis draws a group at %q and no card carries a kind", child.Value)
		}
	}
}

// TestABadChainIsRefusedByItsOwnName asserts that three names cover the four
// bad chains, and that only the one naming an unknown word lists the axes.
// One name over all four would print a sentence calling a legal axis not an
// axis and then listing it as one.
func TestABadChainIsRefusedByItsOwnName(t *testing.T) {
	h := newHarness(t)
	axes := strings.Join(GroupAxes(), ", ")
	cases := []struct {
		chain   []string
		refusal string
		detail  string
		lists   bool
	}{
		{chain: []string{"priority"}, refusal: contract.UnknownAxis, detail: "priority", lists: true},
		{chain: []string{FieldAt}, refusal: contract.UnknownAxis, detail: FieldAt, lists: true},
		{chain: []string{FieldState, FieldState}, refusal: contract.RepeatedAxis, detail: FieldState},
		{
			chain:   []string{FieldState, FieldSubstate, FieldHolder, FieldActor, FieldEvent},
			refusal: contract.ChainTooLong,
			detail:  "5",
		},
	}
	for _, c := range cases {
		_, err := h.library.Tree(&Request{Verb: "tree"}, c.chain, LevelCards)
		refusal, ok := err.(*contract.Refusal)
		if !ok {
			t.Fatalf("%v: wanted a refusal, got %v", c.chain, err)
		}
		if refusal.Name != c.refusal {
			t.Errorf("%v refuses with %s, want %s", c.chain, refusal.Name, c.refusal)
		}
		if refusal.Detail != c.detail {
			t.Errorf("%v names %q in its detail, want %q", c.chain, refusal.Detail, c.detail)
		}
		if listed := refusal.Extra["axes"] == axes; listed != c.lists {
			t.Errorf("%v lists the axes: %v, want %v", c.chain, listed, c.lists)
		}
		if c.refusal == contract.ChainTooLong {
			if refusal.Extra["asked"] != "5" || refusal.Extra["allowed"] != "4" {
				t.Errorf("the over-long chain names %q and %q, want 5 and 4", refusal.Extra["asked"], refusal.Extra["allowed"])
			}
			for _, axis := range GroupAxes() {
				if strings.Contains(strings.Join(values(refusal.Extra), " "), axis) {
					t.Errorf("the over-long chain names the axis %s and it has no offending word to name", axis)
				}
			}
		}
	}
}

// values lists a refusal's named values, for an assertion about what a
// sentence cannot carry.
func values(extra map[string]string) []string {
	var carried []string
	for _, value := range extra {
		carried = append(carried, value)
	}
	return carried
}

// TestEachCommandRefusesADepthAgainstItsOwnLadder asserts that a level one
// command declares and the other does not is refused, and that the sentence
// lists the ladder of the command that refused rather than the union of both.
func TestEachCommandRefusesADepthAgainstItsOwnLadder(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card to walk from")
	cases := []struct {
		what   string
		level  string
		levels []string
		run    func(level string) error
	}{
		{what: "tree", level: LevelEntities, levels: TreeLevels, run: func(level string) error {
			_, err := h.library.Tree(&Request{Verb: "tree"}, nil, level)
			return err
		}},
		{what: "tree", level: "3", levels: TreeLevels, run: func(level string) error {
			_, err := h.library.Tree(&Request{Verb: "tree"}, nil, level)
			return err
		}},
		{what: "contents", level: LevelGroups, levels: ContentsLevels, run: func(level string) error {
			_, err := h.library.Contents(&Request{Verb: "contents", Ref: ref}, level)
			return err
		}},
	}
	for _, c := range cases {
		refusal, ok := c.run(c.level).(*contract.Refusal)
		if !ok {
			t.Fatalf("%s at %s: wanted a refusal", c.what, c.level)
		}
		if refusal.Name != contract.UnknownDepth {
			t.Errorf("%s at %s refuses with %s, want %s", c.what, c.level, refusal.Name, contract.UnknownDepth)
		}
		if refusal.Detail != c.level {
			t.Errorf("%s at %s names %q in its detail", c.what, c.level, refusal.Detail)
		}
		if want := strings.Join(c.levels, ", "); refusal.Extra["levels"] != want {
			t.Errorf("%s at %s lists the levels %q, want %q", c.what, c.level, refusal.Extra["levels"], want)
		}
	}
}

// TestEveryReferenceAContentsTreeDrawsResolves asserts that a label copied out
// of the tree opens the thing it names, and that a node below a card carries
// the card's own reference in front of it.
func TestEveryReferenceAContentsTreeDrawsResolves(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card with things below it")
	h.comment(ref, "a note")
	h.attach(ref, "notes.txt", "some bytes")
	writeItem(t, h.card(ref).Dir, "AC-1", 1)
	h.reopen()

	built := contentsOf(t, h, "workbench", LevelAll)
	drawn := 0
	walkTree(built.Root, func(node TreeNode) {
		if node.Kind == bench.KindWorkbench {
			return
		}
		drawn++
		if node.Ref == "" {
			t.Errorf("the %s node carries no reference", node.Kind)
			return
		}
		path, err := h.library.Bench.ResolvePath(node.Ref)
		if err != nil {
			t.Errorf("%s does not resolve: %v", node.Ref, err)
			return
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s resolves to %s and nothing is there: %v", node.Ref, path, err)
		}
	})
	if drawn == 0 {
		t.Fatal("the walk drew nothing below the workbench, so this test proves nothing")
	}
	card := h.card(ref)
	for _, node := range findKind(built.Root, bench.KindComment) {
		if !strings.HasPrefix(node.Ref, card.Ref(h.library.Bench.Slug)+"/") {
			t.Errorf("the comment reference %q does not carry its card's reference in front of it", node.Ref)
		}
	}
}

// findKind collects every node of a tree carrying one kind.
func findKind(root TreeNode, kind string) []TreeNode {
	var found []TreeNode
	walkTree(root, func(node TreeNode) {
		if node.Kind == kind {
			found = append(found, node)
		}
	})
	return found
}

// TestAnEntityWithNothingBelowItIsAnAnswer asserts that a containment root
// that is itself a leaf succeeds, counting nothing and hiding nothing.
func TestAnEntityWithNothingBelowItIsAnAnswer(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card with nothing below it")
	built := contentsOf(t, h, ref, LevelEntities)
	if built.Root.Count != 0 {
		t.Errorf("the card counts %d and carries nothing", built.Root.Count)
	}
	if len(built.Root.Children) != 0 {
		t.Errorf("the card draws %d children and carries none", len(built.Root.Children))
	}
	if built.Root.Hidden != nil {
		t.Errorf("the card reports %v and hides nothing", built.Root.Hidden)
	}
}

// TestTheWorkbenchWalkDrawsItsCollectionsInOrder asserts the order of the
// containment walk from the workbench root, and that a comment's own
// attachments are reported through the account rather than drawn at the
// default depth.
func TestTheWorkbenchWalkDrawsItsCollectionsInOrder(t *testing.T) {
	h := newHarness(t)
	first := h.add("the first card filed")
	second := h.add("the second card filed")
	h.comment(first, "a note")
	comments, err := bench.Comments(h.card(first).Dir)
	if err != nil || len(comments) != 1 {
		t.Fatalf("read the comment back: %v", err)
	}
	if _, err := bench.AddAttachment(comments[0].Dir, writeFile(t, "evidence.txt"), "", "test"); err != nil {
		t.Fatalf("attach to the comment: %v", err)
	}
	h.reopen()

	cards := contentsOf(t, h, "workbench", LevelCards)
	var kinds []string
	for _, child := range cards.Root.Children {
		kinds = append(kinds, child.Kind)
	}
	wanted := []string{
		bench.KindState, bench.KindState, bench.KindState, bench.KindState, bench.KindState,
		bench.KindCard, bench.KindCard,
	}
	if strings.Join(kinds, ",") != strings.Join(wanted, ",") {
		t.Errorf("the workbench draws %v, and the states come in flow order before the cards", kinds)
	}
	var drawn []string
	for _, child := range cards.Root.Children {
		if child.Kind == bench.KindCard {
			drawn = append(drawn, child.Ref)
		}
	}
	if strings.Join(drawn, ",") != first+","+second {
		t.Errorf("the cards draw as %v and they arrived as %s then %s", drawn, first, second)
	}

	entities := contentsOf(t, h, "workbench", LevelEntities)
	comment := findKind(entities.Root, bench.KindComment)
	if len(comment) != 1 {
		t.Fatalf("the walk drew %d comments and the fixture wrote one", len(comment))
	}
	if len(comment[0].Children) != 0 {
		t.Errorf("the comment draws %d children at the entities level, and its own attachments sit past it", len(comment[0].Children))
	}
	if comment[0].Hidden == nil || comment[0].Hidden.Children != 1 {
		t.Fatalf("the comment reports %v and it is holding one attachment back", comment[0].Hidden)
	}
	if comment[0].Count != 1 {
		t.Errorf("the comment counts %d and contains one attachment", comment[0].Count)
	}
}

// writeFile writes a file outside the workbench for an attachment to copy.
func writeFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("some bytes\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
