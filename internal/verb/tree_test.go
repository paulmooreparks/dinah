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

// groupValues names every group a node draws, for a failure message and for the
// tests that pin which states a column heads.
//
// A node's children are a mix of group nodes and card leaves wherever a state is
// shown without a heading, and a card leaf carries no Value at all, so reading
// every child's Value would report an empty string for each inlined card and
// make "the states drawn" mean something wider than it says. Only group nodes
// are counted here.
func groupValues(node TreeNode) []string {
	var values []string
	for _, child := range node.Children {
		if child.Kind != NodeGroup {
			continue
		}
		values = append(values, child.Value)
	}
	return values
}

// leafChildRefs collects the reference of every card node attached directly to
// one node, which is where a state shown without a heading puts its cards. It
// does not descend, so a card sitting under a group node of its own is not one
// of these.
func leafChildRefs(node TreeNode) []string {
	var refs []string
	for _, child := range node.Children {
		if child.Kind != bench.KindCard {
			continue
		}
		refs = append(refs, child.Ref)
	}
	return refs
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
	first := h.ready("held by one")
	second := h.ready("held by another")
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
//
// The four cards stand at the intake column, which takes no work up and so
// declares no state. Their state draws no heading, so there is no group node
// between them and the column and nothing at that level to carry the depth
// report. The column becomes the boundary instead. Its cards belong to the rank
// the state axis would have occupied rather than to the rank their position in
// its child list suggests, so a groups-level read holds them back exactly as it
// holds back the cards under a heading, and the column accounts for them in its
// own Hidden.
func TestDepthReportsAtTheBoundaryAndNowhereElse(t *testing.T) {
	h := newHarness(t)
	for range 4 {
		h.add("a card of the intake")
	}
	built := treeOf(t, h, "", nil, LevelGroups)

	column := groupAt(t, built.Root, "intake")
	if column.Count != 4 {
		t.Errorf("the column counts %d cards and holds 4", column.Count)
	}
	if len(column.Children) != 0 {
		t.Errorf("the column draws %v and the depth left it nothing to draw", groupValues(column))
	}
	if column.Hidden == nil {
		t.Fatalf("the column reports nothing and the depth cut its cards off")
	}
	if got := strings.Join(column.Hidden.Reason, ","); got != ReasonDepth {
		t.Errorf("the column reports the reasons %q, want %q", got, ReasonDepth)
	}
	if column.Hidden.Children != 4 || column.Hidden.Subjects != 4 {
		t.Errorf("the column reports %d children and %d subjects, want 4 and 4",
			column.Hidden.Children, column.Hidden.Subjects)
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
	held := h.ready("the card the holder is working")
	h.add("a card nobody holds")
	h.mustDo(&Request{Verb: Claim, Actor: "zoya", Card: held, Holder: "zoya"})

	built := treeOf(t, h, "state:ready", []string{FieldHolder}, LevelCards)
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
		h.ready("a ready card of the aftercare station")
	}
	held := h.ready("the card the holder is working")
	h.mustDo(&Request{Verb: Claim, Actor: "zoya", Card: held, Holder: "zoya"})

	built := treeOf(t, h, "state:ready", nil, LevelGroups)
	column := groupAt(t, built.Root, aftercareSlug)
	if column.Hidden == nil {
		t.Fatalf("the column reports nothing and the filter removed a card below it")
	}
	if got := strings.Join(column.Hidden.Reason, ","); got != ReasonFilter {
		t.Errorf("the column reports the reasons %q, and its own children are all drawn, want %q", got, ReasonFilter)
	}
	ready := groupAt(t, column, contract.StateReady)
	if ready.Hidden == nil {
		t.Fatalf("the ready group reports nothing and the depth cut its cards off")
	}
	if got := strings.Join(ready.Hidden.Reason, ","); got != ReasonDepth {
		t.Errorf("the ready group reports the reasons %q, want %q", got, ReasonDepth)
	}
	active := groupAt(t, column, contract.StateActive)
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
		h.ready("a ready card")
	}
	held := h.ready("the card the holder is working")
	h.mustDo(&Request{Verb: Claim, Actor: "zoya", Card: held, Holder: "zoya"})

	built := treeOf(t, h, "state:ready", []string{FieldColumn}, LevelGroups)
	column := groupAt(t, built.Root, aftercareSlug)
	if column.Hidden == nil {
		t.Fatalf("the column reports nothing and it is holding two kinds of thing back")
	}
	want := []string{ReasonDepth, ReasonFilter}
	if got := strings.Join(column.Hidden.Reason, ","); got != strings.Join(want, ",") {
		t.Errorf("the column names the reasons %q and the order is fixed at %q", got, strings.Join(want, ","))
	}
	if column.Hidden.Children != 3 {
		t.Errorf("the column reports %d children cut off and the depth cut off 3", column.Hidden.Children)
	}
	if column.Hidden.Subjects != 3 {
		t.Errorf("the column reports %d surviving subjects below and 3 survived", column.Hidden.Subjects)
	}
	if column.Hidden.Filtered != 1 {
		t.Errorf("the column reports %d filtered out and the filter removed 1", column.Hidden.Filtered)
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
	// The blocked group is drawn only where a card stands blocked, and this
	// fixture blocks nothing, so the group is absent rather than empty. The
	// walk above is what asserts the tree hides nothing; this reads the
	// suppression, which the walk cannot see because a group that was never
	// drawn reports no account of itself either.
	if drawn := groupValues(groupAt(t, grouped.Root, "intake")); contains(drawn, contract.StateBlocked) {
		t.Errorf("intake draws the groups %v and the fixture blocks nothing", drawn)
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
	split := h.ready("moved into doing by one owner and claimed later by another")
	h.mustDo(&Request{Verb: Claim, Actor: "zoya", Card: split, Holder: "zoya"})
	h.mustDo(&Request{Verb: Move, Actor: "zoya", Card: split, Column: doing})
	h.mustDo(&Request{Verb: Move, Actor: "zoya", Card: split, Column: review})
	h.mustDo(&Request{Verb: Release, Actor: "zoya", Card: split})
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: split, Holder: "alka"})
	h.add("a card nobody has moved")

	witnessed := h.ready("moved into doing by the same owner who claimed it")
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: witnessed, Holder: "alka"})
	h.mustDo(&Request{Verb: Move, Actor: "alka", Card: witnessed, Column: doing})

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
		t.Errorf("the table reads %s on %d rows, and %s and %s are the only two axes that enumerate", EnumerationClosed, closed, FieldColumn, FieldState)
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
	first := h.ready("the first active card")
	second := h.ready("the second active card")
	h.add("a ready card of the intake")
	for _, ref := range []string{first, second} {
		h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: ref, Holder: "alka"})
		h.mustDo(&Request{Verb: Move, Actor: "alka", Card: ref, Column: review})
	}

	built := treeOf(t, h, "state:ready", nil, LevelCards)
	if built.Root.Count != 1 || built.Root.Hidden == nil || built.Root.Hidden.Filtered != 2 {
		t.Fatalf("the root counts %d and reports %v, and the workbench holds three cards of which one matches", built.Root.Count, built.Root.Hidden)
	}
	column := groupAt(t, built.Root, "review")
	if column.Count != 0 {
		t.Errorf("the Review group counts %d and the filter left it nothing", column.Count)
	}
	if column.Hidden == nil || column.Hidden.Filtered != 2 {
		t.Fatalf("the Review group reports %v and the filter removed two cards from it", column.Hidden)
	}
	ready := groupAt(t, column, contract.StateReady)
	if ready.Count != 0 {
		t.Errorf("the ready group inside Review counts %d and holds nothing", ready.Count)
	}
	if ready.Hidden != nil {
		t.Errorf("the ready group inside Review reports %v and it hides nothing at all", ready.Hidden)
	}
}

// TestTheDefaultChainDrawsTheStatusTree asserts that a bare tree nests column
// over state over cards, enumerates both closed axes completely and in
// their declared order, and orders the cards of a group the way a listing of
// that column orders them.
func TestTheDefaultChainDrawsTheStatusTree(t *testing.T) {
	h := newHarness(t)
	first := h.add("the first card filed")
	second := h.add("the second card filed")
	h.renumber(h.card(second).ID, 1)
	h.renumber(h.card(first).ID, 2)
	h.reopen()

	built := treeOf(t, h, "", nil, LevelCards)
	var columns []string
	for _, child := range built.Root.Children {
		columns = append(columns, child.Value)
	}
	want := []string{"intake", "doing", "review", aftercareSlug, "finished", "closed"}
	if strings.Join(columns, ",") != strings.Join(want, ",") {
		t.Errorf("the columns draw as %v and the flow declares them %v", columns, want)
	}
	// The two closed axes enumerate differently, and the state axis does
	// not enumerate the same set under every column. Intake takes no work up,
	// so it declares no state at all and heads no state group whatever stands
	// there. Its two cards hang off the column as bare leaves, which is why the
	// loop below wants nothing from the intake half and the leaf check further
	// down still finds both cards. Doing takes work up and is what
	// discriminates here: it declares ready and active and heads both, so a
	// rule that dropped the declared-group promise from every column on the
	// board would redden the doing half while leaving the intake half alone.
	intakeGroup := groupAt(t, built.Root, "intake")
	for _, c := range []struct {
		group TreeNode
		want  []string
	}{
		{group: intakeGroup, want: nil},
		{
			group: groupAt(t, built.Root, "doing"),
			want:  []string{contract.StateReady, contract.StateActive},
		},
	} {
		states := groupValues(c.group)
		if strings.Join(states, ",") != strings.Join(c.want, ",") {
			t.Errorf("the states of the %s group draw as %v and the order is fixed at %v",
				c.group.Value, states, c.want)
		}
	}
	listing, err := h.library.List(&Request{Verb: "ls", Column: "intake"})
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
	if inline := leafChildRefs(intakeGroup); strings.Join(inline, ",") != strings.Join(listed, ",") {
		t.Errorf("the column attaches %v directly and every card standing at it is %v", inline, listed)
	}
}

// TestAnActPlaneAxisReadsTheWholeActRecord asserts that a card two owners have
// acted on is drawn under both, and that the filter's own witness does not
// narrow where grouping puts a card.
func TestAnActPlaneAxisReadsTheWholeActRecord(t *testing.T) {
	h := newHarness(t)
	shared := h.ready("a card two owners have acted on")
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

// TestOnlyTheTwoClosedAxesEnumerateTheirMembers asserts that a column nobody
// has entered draws a group under column and none under entered, which is what
// separates a closed axis from a journal-shaped one.
func TestOnlyTheTwoClosedAxesEnumerateTheirMembers(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("a card that has only ever been in two columns")
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: ref, Holder: "alka"})
	h.mustDo(&Request{Verb: Move, Actor: "alka", Card: ref, Column: doing})

	byColumn := treeOf(t, h, "", []string{FieldColumn}, LevelCards)
	if len(byColumn.Root.Children) != 6 {
		t.Errorf("the column axis draws %d groups and the workbench declares six columns", len(byColumn.Root.Children))
	}
	byEntered := treeOf(t, h, "", []string{FieldEntered}, LevelCards)
	var entered []string
	for _, child := range byEntered.Root.Children {
		entered = append(entered, child.Value)
	}
	sort.Strings(entered)
	if strings.Join(entered, ",") != "aftercare,doing,intake" {
		t.Errorf("the entered axis draws groups for %v, and the card has entered only two columns", entered)
	}
	blocks := treeOf(t, h, "", []string{FieldBlockKind}, LevelCards)
	for _, child := range blocks.Root.Children {
		if child.Value != "" {
			t.Errorf("the block_kind axis draws a group at %q and no card carries a kind", child.Value)
		}
	}
}

// standAt writes a card's position straight into its anchor, which is how a
// test builds a card standing at a value the workbench does not declare. No
// verb produces one, and the file the tool reads is a file a person edits.
func standAt(t *testing.T, h *harness, ref, column, state string) {
	t.Helper()
	card := h.card(ref)
	card.Column, card.State = column, state
	if err := card.Save(); err != nil {
		t.Fatalf("save %s: %v", ref, err)
	}
	h.reopen()
}

// TestNoCardFallsOutOfAClosedAxisTree asserts that a closed axis draws a group
// for every value some card carries as well as for every value the workbench
// declares, and draws the undeclared values after the declared ones.
//
// A tree drawing only the declared members loses the cards carrying anything
// else, and loses them silently: no group, no account, and a root above them
// still counting them. Both closed axes can be reached without corrupt data,
// so the fixture builds one card on each. A column deleted from the flow leaves
// the cards standing in it naming a column nothing declares, and a state
// written by hand outside the three is a value the reader of the anchor passes
// through. The assertion is that the leaves add up to the root rather than
// that a group appeared, since a tree can grow a group and still drop a card.
func TestNoCardFallsOutOfAClosedAxisTree(t *testing.T) {
	h := newHarness(t)
	const (
		deletedColumn  = "a00000000009"
		handState      = "paused"
		cardsFiled     = 3
		declaredColumn = intake
	)
	declared := h.add("a card standing where the workbench says it may")
	undeclaredColumn := h.add("a card standing in a column the flow no longer declares")
	standAt(t, h, undeclaredColumn, deletedColumn, contract.StateReady)
	undeclaredState := h.add("a card carrying a state the tool never writes")
	standAt(t, h, undeclaredState, declaredColumn, handState)

	for _, c := range []struct {
		axis  string
		value string
		// declared is a lower bound on where the undeclared group may be
		// drawn rather than the count of groups the axis draws, because the
		// question here is that the undeclared value comes after the
		// declared ones and not how many of those there are.
		declared int
		// absent is a value this axis must draw no group at, empty where the
		// axis has none. It carries what the lower bound above cannot: a
		// bound that only says "not before position two" is met by a tree
		// drawing a blocked group as well, and the count moved from three to
		// two precisely because that group is no longer drawn.
		absent string
	}{
		{axis: FieldColumn, value: deletedColumn, declared: 5},
		// Two rather than three: the state axis grouped standalone
		// enumerates the union of what the fixture's columns can hold, which
		// is all three, and no card here stands blocked, so the blocked
		// group is not drawn and the undeclared value follows ready and
		// active.
		{axis: FieldState, value: handState, declared: 2, absent: contract.StateBlocked},
	} {
		built := treeOf(t, h, "", []string{c.axis}, LevelCards)
		if built.Root.Count != cardsFiled {
			t.Fatalf("the %s tree counts %d cards at its root and the workbench holds %d", c.axis, built.Root.Count, cardsFiled)
		}
		drawn := map[string]bool{}
		total := 0
		for _, group := range built.Root.Children {
			total += group.Count
			for _, card := range group.Children {
				drawn[card.Ref] = true
			}
		}
		if total != cardsFiled {
			t.Errorf("the %s groups count %d cards between them and the root counts %d, so the tree lost one", c.axis, total, cardsFiled)
		}
		for _, ref := range []string{declared, undeclaredColumn, undeclaredState} {
			if !drawn[ref] {
				t.Errorf("%s is drawn nowhere in the %s tree, and the root counts it", ref, c.axis)
			}
		}
		if c.absent != "" && groupPosition(built.Root, c.absent) >= 0 {
			t.Errorf("the %s tree draws a group at %q and no card of the fixture stands there", c.axis, c.absent)
		}
		at := groupPosition(built.Root, c.value)
		if at < 0 {
			t.Errorf("the %s tree draws no group at %q", c.axis, c.value)
			continue
		}
		if at < c.declared {
			t.Errorf("the %s tree draws the undeclared group %q at position %d, ahead of the %d values the workbench declares", c.axis, c.value, at, c.declared)
		}
	}
}

// groupPosition is where a tree draws one group value, or -1 when it draws no
// group at that value.
func groupPosition(root TreeNode, value string) int {
	for at, child := range root.Children {
		if child.Kind == NodeGroup && child.Value == value {
			return at
		}
	}
	return -1
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
		{chain: []string{"urgency"}, refusal: contract.UnknownAxis, detail: "urgency", lists: true},
		{chain: []string{FieldAt}, refusal: contract.UnknownAxis, detail: FieldAt, lists: true},
		{chain: []string{FieldColumn, FieldColumn}, refusal: contract.RepeatedAxis, detail: FieldColumn},
		{
			chain:   []string{FieldColumn, FieldState, FieldHolder, FieldActor, FieldEvent},
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

// TestACardIsReachedByItsOwnAddressAndByNoOther asserts that a reference
// descending from the workbench into its cards or its columns is refused, and
// that the reference the walk actually draws for each of them still opens it.
//
// The containment grammar mounts cards and columns under the workbench, so a
// resolver walking that grammar from the root accepts `<slug>/cards/1` unless
// something stops it. Nothing prints that form: the walk draws a card by its
// own reference and a column by its slug. What the form did produce was an
// answer marked as a card with no card in it, which crashed `dinah contents`
// on the nil and, through `dinah attach`, wrote a card's own history into the
// workbench journal under the workbench's lock. One entity with two spellings
// has to be kept equal everywhere it is read, and it was not.
//
// The attachments collection is the one the widening was for, and it stays
// open, because `<slug>/attachments/1` is a reference the walk prints and no
// other spelling reaches it.
func TestACardIsReachedByItsOwnAddressAndByNoOther(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card the workbench mounts")
	h.attach("workbench", "policy.txt", "the workbench's own bytes")
	h.reopen()
	slug := h.library.Bench.Slug

	for _, spelling := range []string{
		slug + "/cards/1",
		"workbench/cards/1",
		slug + "/cards/1/comments/1",
		slug + "/columns/1",
		"workbench/columns/1",
	} {
		built, err := h.library.Contents(&Request{Verb: "contents", Ref: spelling}, LevelEntities)
		if err == nil {
			t.Errorf("contents %s answered with a tree rooted at %s, and no walk draws that reference", spelling, built.Root.Kind)
			continue
		}
		refusal, ok := err.(*contract.Refusal)
		if !ok {
			t.Errorf("contents %s failed with %v, which is not a refusal", spelling, err)
			continue
		}
		if refusal.Name != contract.UnknownPath {
			t.Errorf("contents %s refuses with %s, want %s", spelling, refusal.Name, contract.UnknownPath)
		}
	}

	drawn := contentsOf(t, h, "workbench", LevelAll)
	for _, node := range findKind(drawn.Root, bench.KindCard) {
		if _, err := h.library.Bench.ResolvePath(node.Ref); err != nil {
			t.Errorf("the walk draws the card as %s and that does not resolve: %v", node.Ref, err)
		}
		if node.Ref != ref {
			t.Errorf("the walk draws the card as %s, want its own reference %s", node.Ref, ref)
		}
	}
	if _, err := h.library.Bench.ResolvePath(slug + "/attachments/1"); err != nil {
		t.Errorf("the workbench's own attachment is drawn as %s/attachments/1 and does not resolve: %v", slug, err)
	}
}

// TestEveryReferenceAContentsTreeDrawsResolves asserts that a label copied out
// of the tree opens the thing it names, and that a node below a card carries
// the card's own reference in front of it.
//
// The assertion is on identity rather than on existence. A resolver that reads
// one collection and discards the rest of a reference returns a real file for
// `<card>/comments/1/attachments/1`, the comment's own anchor, so asking
// whether some file came back is answered by the defect as readily as by the
// fix. Every entity of the grammar keeps its anchor in a directory named for
// its identifier, so the directory the resolution lands in is the node itself.
// The workbench root carries no such directory; its own anchor sits directly
// under the bench root, so its identity check compares the resolved path
// against that anchor rather than against a node ID it does not have.
//
// The fixture builds every two-level case the grammar has: an attachment under
// a comment, and one under each of the two kinds the workbench mounts
// directly. A fixture reaching only one level down cannot fail against a
// resolver that stops after one level. It includes the root, per dinah-151
// OQ-9: the root draws the address the tool already accepts for the
// workbench, so it belongs in the corpus rather than out of it.
func TestEveryReferenceAContentsTreeDrawsResolves(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card with things below it")
	h.comment(ref, "a note")
	h.attach(ref, "notes.txt", "some bytes")
	h.attach(ref+"/comments/1", "evidence.txt", "the comment's own bytes")
	h.attach("workbench", "policy.txt", "the workbench's own bytes")
	h.attach(h.library.Bench.Columns[0].Ref(), "station.txt", "a column's own bytes")
	writeItem(t, h.card(ref).Dir, "AC-1", 1)
	h.reopen()

	built := contentsOf(t, h, "workbench", LevelAll)
	drawn := 0
	deep := 0
	root := false
	walkTree(built.Root, func(node TreeNode) {
		drawn++
		if collectionsBelowTheHead(node.Ref) > 1 {
			deep++
		}
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
			return
		}
		if node.Kind == bench.KindWorkbench {
			root = true
			if want := filepath.Join(h.library.Bench.Root, bench.WorkbenchAnchor); path != want {
				t.Errorf("%s resolves to %s rather than the workbench's own anchor %s", node.Ref, path, want)
			}
			return
		}
		if reached := filepath.Base(filepath.Dir(path)); reached != node.ID {
			t.Errorf("%s names the %s %s and opens %s instead", node.Ref, node.Kind, node.ID, reached)
		}
	})
	if drawn == 0 {
		t.Fatal("the walk drew nothing below the workbench, so this test proves nothing")
	}
	if !root {
		t.Fatal("the walk never visited the workbench root, so this test proves nothing about it")
	}
	if deep == 0 {
		t.Fatal("the walk drew no reference descending through two collections, so this test proves nothing about a resolver that stops after one")
	}
	card := h.card(ref)
	for _, node := range findKind(built.Root, bench.KindComment) {
		if !strings.HasPrefix(node.Ref, card.Ref(h.library.Bench.Slug)+"/") {
			t.Errorf("the comment reference %q does not carry its card's reference in front of it", node.Ref)
		}
	}
}

// TestAWalkRootedAtAReferenceDrawsThatReferenceBack covers the class the
// cycle-2 blocker and the empty containment header both belong to: a resolver
// widened at one root that fills only the fields its older callers read. The
// guard above holds the references a walk from the workbench root composes,
// and this one holds the other direction. It roots a walk at each of those
// references and requires the root node to carry the same address back, so a
// field the resolver stops filling under any head reddens here rather than
// reaching a reader as a pair of empty parentheses.
//
// The workbench root is included, per dinah-151 OQ-9: the root now draws the
// address the tool already accepts for the workbench, so a walk rooted at
// that address must draw the workbench itself back, exactly as a walk rooted
// at any other node's reference must.
func TestAWalkRootedAtAReferenceDrawsThatReferenceBack(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card with things below it")
	h.comment(ref, "a note")
	h.attach(ref, "notes.txt", "some bytes")
	h.attach(ref+"/comments/1", "evidence.txt", "the comment's own bytes")
	h.attach("workbench", "policy.txt", "the workbench's own bytes")
	h.attach(h.library.Bench.Columns[0].Ref(), "station.txt", "a column's own bytes")
	writeItem(t, h.card(ref).Dir, "AC-1", 1)
	h.reopen()

	drawn := map[string]int{}
	walkTree(contentsOf(t, h, "workbench", LevelAll).Root, func(node TreeNode) {
		drawn[node.Kind]++
		rooted, err := h.library.Contents(&Request{Verb: "contents", Ref: node.Ref}, LevelRoot)
		if err != nil {
			t.Errorf("a walk rooted at the %s %s: %v", node.Kind, node.Ref, err)
			return
		}
		if rooted.Root.Ref != node.Ref {
			t.Errorf("a walk rooted at %s draws its root's reference as %q", node.Ref, rooted.Root.Ref)
		}
		if rooted.Root.Kind != node.Kind {
			t.Errorf("a walk rooted at %s draws a %s rather than a %s", node.Ref, rooted.Root.Kind, node.Kind)
		}
	})
	// The kinds are named rather than counted, because the defect this guard
	// exists for reached one kind under two heads and no other, so a corpus
	// that happens to miss a head proves nothing about it.
	for _, kind := range []string{bench.KindWorkbench, bench.KindColumn, bench.KindCard, bench.KindComment, bench.KindItem, bench.KindAttachment} {
		if drawn[kind] == 0 {
			t.Fatalf("the walk drew no %s, so this test proves nothing about that kind", kind)
		}
	}
	// The two heads the fix is about: an attachment reached below the
	// workbench and one reached below a column, neither of which belongs to a
	// card and both of which the older composer left with no reference.
	for _, below := range []string{"workbench/attachments/1", h.library.Bench.Columns[0].Ref() + "/attachments/1"} {
		entity, err := h.library.Bench.ResolveEntity(below)
		if err != nil {
			t.Errorf("%s does not resolve: %v", below, err)
			continue
		}
		if entity.Ref == "" {
			t.Errorf("%s resolves to an answer carrying no reference", below)
		}
	}
}

// collectionsBelowTheHead counts the collections a reference descends through,
// which is the depth the resolver has to recurse to in order to answer it.
//
// A reference is a head naming an entity a person addresses directly, then a
// pair of segments per collection: the collection's own name and a position in
// it. So the number of pairs is the number of collections, and counting the
// slashes instead answers 2 for `fx-1/comments/1`, which is one collection
// deep and is exactly the reference the fixture this counter rejects would
// have drawn.
//
// The measure floors on two shapes the walk never draws: a bare collection
// such as `fx-1/comments` answers 0 while descending one, and a payload such
// as `fx-1/attachments/1/payload` answers 1 against three segments. Both
// under-count, so the guard reading this fires more readily rather than less,
// which is the direction a safety catch may err in and the direction the slash
// count did not.
func collectionsBelowTheHead(ref string) int {
	_, rest, found := strings.Cut(ref, "/")
	if !found || rest == "" {
		return 0
	}
	return len(strings.Split(rest, "/")) / 2
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
		bench.KindColumn, bench.KindColumn, bench.KindColumn,
		bench.KindColumn, bench.KindColumn, bench.KindColumn,
		bench.KindCard, bench.KindCard,
	}
	if strings.Join(kinds, ",") != strings.Join(wanted, ",") {
		t.Errorf("the workbench draws %v, and the columns come in flow order before the cards", kinds)
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

// nobodyWorksHereDefinition is a flow where nobody takes work up at any column:
// one intake column and one done column, and nothing between them. Neither
// column declares a state, so the union across the workbench is empty, and a
// card standing on this flow can stand ready or blocked and can never stand
// active. That is what makes it the one fixture able to tell the union rule
// apart from a rule that draws all three states whatever the workbench says.
const nobodyWorksHereDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Nobody works here",
  "instructions": "The standing text of this workbench.\n",
  "columns": [
    { "id": "c00000000001", "title": "Waiting", "kind": "intake",
      "instructions": "Waiting instructions.\n" },
    { "id": "c00000000002", "title": "Filed", "kind": "done",
      "instructions": "Filed instructions.\n" }
  ]
}`

// TestStateResolvesTheColumnAboveItWhateverAxisComesBetween asserts that the
// column an ancestor group fixed still governs the state enumeration when
// another axis stands between the two in the chain.
//
// Grouping partitions a card set and never merges two of them back together,
// so every card below an intake group stands at the intake column however many
// axes intervene, and the state enumeration there is the intake column's own.
// That column declares nothing, so it heads no ready group and the ready card
// hangs off the entered group as a bare leaf. The fixture blocks one of its two
// cards so that the one heading left, blocked, discriminates two ways at once: a
// tree that lost the column would fall back to the union across the whole
// workbench and head ready and active as well, and a tree that suppressed
// blocked wherever it appeared would head nothing.
//
// The leaf comes before the heading, because an un-headed state keeps the place
// in the order its heading would have taken and ready precedes blocked.
//
// The same tree taken with the column axis dropped out of the chain is the
// control. Without it the state groups draw active as well, which is what
// makes the absence of active above a statement about the intervening axis
// rather than about a workbench that never draws active at all.
func TestStateResolvesTheColumnAboveItWhateverAxisComesBetween(t *testing.T) {
	h := newHarness(t)
	standing := h.add("a card standing ready where it was filed")
	stopped := h.add("a card stopped where it was filed")
	h.mustDo(&Request{Verb: Block, Card: stopped, Actor: "alka", Reason: "waiting on a ruling"})

	built := treeOf(t, h, "", []string{FieldColumn, FieldEntered, FieldState}, LevelCards)
	entered := groupAt(t, groupAt(t, built.Root, "intake"), "intake")
	want := []string{contract.StateBlocked}
	if drawn := groupValues(entered); strings.Join(drawn, ",") != strings.Join(want, ",") {
		t.Errorf("the states under the entered group draw as %v, and the intake column heads %v", drawn, want)
	}
	if inline := leafChildRefs(entered); strings.Join(inline, ",") != standing {
		t.Errorf("the entered group attaches %v directly, and the card standing ready there is %s", inline, standing)
	}
	if len(entered.Children) != 2 {
		t.Fatalf("the entered group draws %d children and holds one ready leaf and one blocked group", len(entered.Children))
	}
	if entered.Children[0].Kind != bench.KindCard || entered.Children[1].Kind != NodeGroup {
		t.Errorf("the entered group draws a %s then a %s, and ready precedes blocked on the state axis",
			entered.Children[0].Kind, entered.Children[1].Kind)
	}
	if refs := leafRefs(groupAt(t, entered, contract.StateBlocked)); strings.Join(refs, ",") != stopped {
		t.Errorf("the blocked group draws %v and the card stopped there is %s", refs, stopped)
	}

	control := treeOf(t, h, "", []string{FieldEntered, FieldState}, LevelCards)
	drawn := groupValues(groupAt(t, control.Root, "intake"))
	if !contains(drawn, contract.StateActive) {
		t.Fatalf("the same fixture draws %v with no column axis in the chain, so the absence of active above proves nothing", drawn)
	}
}

// TestStandaloneStateUnionsAcrossColumns asserts that a state axis grouped
// with no column above it anywhere in the chain enumerates the union of what the
// workbench's columns can hold, rather than the three the contract names.
//
// The fixture is a flow where nobody takes work up at any column, so every
// column declares nothing, the union is empty, and no card on the workbench can
// ever stand active. Nothing here is headed by a declaration, and the root is
// the enclosing node the un-headed rule names when the chain groups by state
// alone. A tree drawing an active group here is drawing a group nothing can
// occupy, which is the defect the first instalment of this ruling fixed, and the
// standard fixture cannot show it: its columns union to all three and pass under
// either rule.
//
// The second half is the control for the first. Blocking a card heads the
// blocked group, so the assertion that the root heads nothing at all while
// nothing is blocked reads as the un-headed rule rather than as a tree that lost
// its groups entirely. Both halves check that the ready card is still drawn, as
// a leaf of the root, because a rule that removed the heading and the card with
// it would be worse than the heading. Active is absent from both halves, and the
// assertion still discriminates: a stateUnion that wrongly returned the
// contract's three whatever the columns declare would head active beside the
// rest and redden the first half.
func TestStandaloneStateUnionsAcrossColumns(t *testing.T) {
	h := harnessFromDefinition(t, "nw", nobodyWorksHereDefinition)
	standing := h.add("a card standing ready")
	stopped := h.add("a card that will be stopped")

	built := treeOf(t, h, "", []string{FieldState}, LevelCards)
	if drawn := groupValues(built.Root); len(drawn) != 0 {
		t.Errorf("the state axis heads %v, and no column on this workbench takes work up or holds a blocked card, so it heads nothing", drawn)
	}
	wantLeaves := []string{standing, stopped}
	if inline := leafChildRefs(built.Root); strings.Join(inline, ",") != strings.Join(wantLeaves, ",") {
		t.Errorf("the root attaches %v directly, and the cards standing ready are %v", inline, wantLeaves)
	}

	h.mustDo(&Request{Verb: Block, Card: stopped, Actor: "alka", Reason: "waiting on a ruling"})
	blocked := treeOf(t, h, "", []string{FieldState}, LevelCards)
	wantBlocked := []string{contract.StateBlocked}
	if drawn := groupValues(blocked.Root); strings.Join(drawn, ",") != strings.Join(wantBlocked, ",") {
		t.Errorf("the state axis heads %v once a card is blocked, and the only state this workbench heads is %v", drawn, wantBlocked)
	}
	if inline := leafChildRefs(blocked.Root); strings.Join(inline, ",") != standing {
		t.Errorf("the root attaches %v directly once the other card is blocked, and the card still standing ready is %s", inline, standing)
	}
}

// TestStateUnderAnUndeclaredColumnFallsBackToTheUnion asserts that a state
// group standing under a column ref the workbench no longer declares draws the
// union, rather than erroring or drawing nothing.
//
// A column deleted from the flow leaves the cards standing in it naming a column
// nothing declares, and no column answers what those cards may carry. The tree
// still has to draw them, so the enumeration falls back to the union across the
// declared columns, and the occupancy rule applies to that union's blocked
// member exactly as it applies to a single column's. Both halves are asserted
// over one card, moved from ready to blocked by hand the same way it was stood
// at the deleted column, so the group appearing is the occupancy rule and not a
// second fixture behaving differently.
func TestStateUnderAnUndeclaredColumnFallsBackToTheUnion(t *testing.T) {
	const deletedColumn = "a00000000009"
	h := newHarness(t)
	stranded := h.add("a card standing in a column the flow no longer declares")
	standAt(t, h, stranded, deletedColumn, contract.StateReady)

	built := treeOf(t, h, "", []string{FieldColumn, FieldState}, LevelCards)
	want := []string{contract.StateReady, contract.StateActive}
	if drawn := groupValues(groupAt(t, built.Root, deletedColumn)); strings.Join(drawn, ",") != strings.Join(want, ",") {
		t.Errorf("the deleted column draws the states %v, and the union less an unoccupied blocked group is %v", drawn, want)
	}

	standAt(t, h, stranded, deletedColumn, contract.StateBlocked)
	blocked := treeOf(t, h, "", []string{FieldColumn, FieldState}, LevelCards)
	wantBlocked := []string{contract.StateReady, contract.StateActive, contract.StateBlocked}
	if drawn := groupValues(groupAt(t, blocked.Root, deletedColumn)); strings.Join(drawn, ",") != strings.Join(wantBlocked, ",") {
		t.Errorf("the deleted column draws the states %v once its card is blocked, and the whole union is %v", drawn, wantBlocked)
	}
}

// TestAnOccupiedBlockedGroupIsDrawnAlongsideAnEmptyActiveGroup asserts the two
// halves of the empty-group rule against each other in one fixture: a blocked
// group is drawn because a card stands in it, and an active group is drawn
// beside it although no card stands in that.
//
// This is the only test in the suite where a card is actually blocked at a
// column that takes work up, so it is the only one that can show the occupancy
// rule drawing a group rather than suppressing one. Unblocking the same card
// and reading the column again is what tells the rule apart from a fixture that
// happens to draw three groups: the blocked group has to leave when its one
// card leaves, and nothing else about the tree may move with it.
func TestAnOccupiedBlockedGroupIsDrawnAlongsideAnEmptyActiveGroup(t *testing.T) {
	h := newHarness(t)
	h.ready("a card standing ready at the station")
	stopped := h.ready("a card stopped at the station")
	h.mustDo(&Request{Verb: Block, Card: stopped, Actor: "alka", Reason: "waiting on a ruling"})

	built := treeOf(t, h, "", nil, LevelCards)
	column := groupAt(t, built.Root, aftercareSlug)
	want := []string{contract.StateReady, contract.StateActive, contract.StateBlocked}
	if drawn := groupValues(column); strings.Join(drawn, ",") != strings.Join(want, ",") {
		t.Fatalf("the station draws the states %v, and it can hold %v with one card blocked", drawn, want)
	}
	if active := groupAt(t, column, contract.StateActive); active.Count != 0 {
		t.Errorf("the active group counts %d and nobody has taken a card up", active.Count)
	}
	if blocked := groupAt(t, column, contract.StateBlocked); blocked.Count != 1 {
		t.Errorf("the blocked group counts %d and one card is blocked", blocked.Count)
	}

	h.mustDo(&Request{Verb: Unblock, Card: stopped, Actor: "alka"})
	after := treeOf(t, h, "", nil, LevelCards)
	drawn := groupValues(groupAt(t, after.Root, aftercareSlug))
	wantAfter := []string{contract.StateReady, contract.StateActive}
	if strings.Join(drawn, ",") != strings.Join(wantAfter, ",") {
		t.Errorf("the station draws the states %v once its card is unblocked, and it holds %v", drawn, wantAfter)
	}
}

// TestAFilteredBlockedGroupIsDrawnOnTheCardsTheFilterRemoved asserts that a
// blocked card the filter cut still draws its group, counting nothing and
// reporting what was filtered.
//
// The occupancy rule reads the cards the filter removed as well as the
// survivors, and the two readings are not the same statement. A column holding a
// blocked card that the reader's own filter hid still has a blocked card, so
// drawing no group there tells the reader the column has none. The honest
// drawing is a group counting nothing with an account of what it is not
// showing, which is what every other emptied group in this tree does.
func TestAFilteredBlockedGroupIsDrawnOnTheCardsTheFilterRemoved(t *testing.T) {
	h := newHarness(t)
	h.ready("a card standing ready at the station")
	stopped := h.ready("a card stopped at the station")
	h.mustDo(&Request{Verb: Block, Card: stopped, Actor: "alka", Reason: "waiting on a ruling"})

	built := treeOf(t, h, "state:ready", nil, LevelCards)
	column := groupAt(t, built.Root, aftercareSlug)
	blocked := groupAt(t, column, contract.StateBlocked)
	if blocked.Count != 0 {
		t.Errorf("the blocked group counts %d and the filter kept none of its cards", blocked.Count)
	}
	if blocked.Hidden == nil || blocked.Hidden.Filtered != 1 {
		t.Fatalf("the blocked group reports %v and the filter removed one card from it", blocked.Hidden)
	}
}

// TestADrawnBlockedGroupKeepsItsDeclaredPlaceAheadOfAHandWrittenState
// asserts where a drawn blocked group sits among the others, which is among
// the values the axis declares and ahead of any value only a card carries.
//
// A group at a value some card carries is drawn whether or not the axis
// declares that value, so an occupied blocked group appears in the tree under
// either rule and its presence alone settles nothing about where the occupancy
// check runs. What the two differ on is order: a blocked group drawn because
// the axis declared it stands with ready and active, while one drawn only
// because a card carries it falls in among the hand-written values, sorted by
// its bytes. The fixture writes a state sorting ahead of blocked so the two
// orders differ, and asserts the whole order rather than a membership.
//
// The filtered half asks the same question of the cards a filter removed. The
// occupancy check reads those as well as the survivors, and a check reading
// only the survivors would drop this blocked group out of the declared values
// and leave it sorted in among the hand-written ones, which the first half
// cannot see because nothing there is filtered.
func TestADrawnBlockedGroupKeepsItsDeclaredPlaceAheadOfAHandWrittenState(t *testing.T) {
	// The value is chosen to sort ahead of blocked. A hand-written state
	// sorting after it would draw the same order under both rules and the
	// test would pass without asking anything.
	const handState = "adjourned"
	h := newHarness(t)
	h.ready("a card standing ready at the station")
	stopped := h.ready("a card stopped at the station")
	h.mustDo(&Request{Verb: Block, Card: stopped, Actor: "alka", Reason: "waiting on a ruling"})
	hand := h.ready("a card carrying a state the tool never writes")
	standAt(t, h, hand, aftercare, handState)

	want := []string{
		contract.StateReady,
		contract.StateActive,
		contract.StateBlocked,
		handState,
	}
	for _, query := range []string{"", "state:ready"} {
		built := treeOf(t, h, query, nil, LevelCards)
		drawn := groupValues(groupAt(t, built.Root, aftercareSlug))
		if strings.Join(drawn, ",") != strings.Join(want, ",") {
			t.Errorf("under the query %q the station draws the states %v, and the declared values come first at %v",
				query, drawn, want)
		}
	}
}

// TestAColumnThatTakesNoWorkUpDrawsNoStateGroupWhenEmpty is dinah-322 AC-3 and
// AC-4. Nobody with access to the workbench claims a card standing at an
// intake column, at a buffer, at a done column, or at a column waiting on
// somebody outside, so a state breakdown beneath one of those tells a reader
// nothing and is not drawn. A station is the contrasting half: it draws ready
// and active whether or not a card stands in either, which is what stops this
// test passing on an implementation that dropped the declared-group promise
// everywhere rather than only where no work is taken up.
//
// The two subtests differ in what stands at the un-worked column. The first
// moves its one card away and leaves those columns empty, which is the case
// dinah-322 closed. The second leaves a card standing at the intake column and
// pins the case dinah-329 closed: the column heads nothing there either, and the
// card hangs off it as a bare leaf instead.
func TestAColumnThatTakesNoWorkUpDrawsNoStateGroupWhenEmpty(t *testing.T) {
	t.Run("an intake column, a buffer and a done column", func(t *testing.T) {
		h := newBufferHarness(t)
		ref := h.add("moved off the queue columns")
		h.at(ref, bufferDoing)

		built := treeOf(t, h, "", []string{FieldColumn, FieldState}, LevelCards)
		for _, column := range []string{bufferIntakeSlug, bufferQueueSlug, bufferDoneSlug} {
			group := groupAt(t, built.Root, column)
			if len(group.Children) != 0 {
				t.Errorf("the %s column draws the state groups %v and holds no card",
					column, groupValues(group))
			}
		}
		doing := groupAt(t, built.Root, bufferDoingSlug)
		want := []string{contract.StateReady, contract.StateActive}
		if got := groupValues(doing); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("the doing column draws the state groups %v and a station always draws %v",
				got, want)
		}
	})

	t.Run("a station waiting on somebody outside", func(t *testing.T) {
		h := newBufferHarness(t)
		h.declare(bufferDoing, "awaiting_outside", "true")
		ref := h.add("waiting on the customer")

		built := treeOf(t, h, "", []string{FieldColumn, FieldState}, LevelCards)
		waiting := groupAt(t, built.Root, bufferDoingSlug)
		if len(waiting.Children) != 0 {
			t.Errorf("the doing column waits on somebody outside and draws the state groups %v",
				groupValues(waiting))
		}
		standing := groupAt(t, built.Root, bufferIntakeSlug)
		if got := groupValues(standing); len(got) != 0 {
			t.Errorf("the intake column holds %s and heads the state groups %v rather than none",
				ref, got)
		}
		if len(standing.Children) != 1 {
			t.Fatalf("the intake column draws %d children and holds the one card %s", len(standing.Children), ref)
		}
		if inline := leafChildRefs(standing); strings.Join(inline, ",") != ref {
			t.Errorf("the intake column attaches %v directly and holds the one card %s", inline, ref)
		}
	})
}

// TestAQueueColumnHeadsBlockedAndInlinesReady is dinah-322 AC-5 as dinah-329
// corrects it. A column where no work is taken up declares no state at all, and
// the two states a card can carry there part company. Blocked keeps a heading,
// because a block is the rare case and it is the one thing a reader most needs
// flagged. Ready gets none, because every card standing at such a column is
// ready and a heading saying so reports the hundred percent case.
//
// The card the heading was removed from is still drawn. It attaches to the
// column as a bare leaf, and this test is what stops an implementation
// satisfying the rule by dropping the card out of a tree whose root count still
// includes it.
//
// Two cards stand at the column, one ready and one blocked, and the order they
// come in is asserted along with their presence. The ready leaf comes first,
// because an un-headed state keeps the place in the state axis its heading would
// have taken, and ready precedes blocked there. Byte order would put blocked
// first, so the assertion discriminates between the axis order and the sort that
// governs a value the axis never heard of.
func TestAQueueColumnHeadsBlockedAndInlinesReady(t *testing.T) {
	h := newBufferHarness(t)
	stopped := h.add("blocked while waiting")
	h.at(stopped, bufferQueue)
	h.mustDo(&Request{Verb: Block, Card: stopped, Actor: "alka", Reason: "waiting on a ruling"})
	waiting := h.add("still waiting its turn")
	h.at(waiting, bufferQueue)

	built := treeOf(t, h, "", []string{FieldColumn, FieldState}, LevelCards)
	group := groupAt(t, built.Root, bufferQueueSlug)
	want := []string{contract.StateBlocked}
	if got := groupValues(group); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("the waiting column heads the state groups %v and two cards stand there, of which only %v earns a heading",
			got, want)
	}
	if inline := leafChildRefs(group); strings.Join(inline, ",") != waiting {
		t.Errorf("the waiting column attaches %v directly and the card standing ready there is %s", inline, waiting)
	}
	if len(group.Children) != 2 {
		t.Fatalf("the waiting column draws %d children and holds one ready leaf and one blocked group", len(group.Children))
	}
	if group.Children[0].Kind != bench.KindCard || group.Children[1].Kind != NodeGroup {
		t.Errorf("the waiting column draws a %s then a %s, and ready precedes blocked on the state axis",
			group.Children[0].Kind, group.Children[1].Kind)
	}
	if refs := leafRefs(groupAt(t, group, contract.StateBlocked)); len(refs) != 1 || refs[0] != stopped {
		t.Errorf("the blocked group of the waiting column draws the cards %v and %s stands there", refs, stopped)
	}
}

// TestStatesDrawnHeadsOnlyDeclaredStatesAndOccupiedBlocked calls the shared
// predicate directly, without a tree around it, so the three cases of the rule
// are pinned where the rule is written rather than only through what the tree
// does with them.
//
// Occupancy is held constant at true throughout, which is what makes the answers
// differ by declaration alone. A column declaring nothing heads neither ready
// nor active however many cards stand in them, heads blocked when a card stands
// blocked, and a column declaring ready heads ready exactly as it did before,
// which is the half that must not move.
func TestStatesDrawnHeadsOnlyDeclaredStatesAndOccupiedBlocked(t *testing.T) {
	standingIn := func(states ...string) func(string) bool {
		occupied := map[string]bool{}
		for _, state := range states {
			occupied[state] = true
		}
		return func(state string) bool { return occupied[state] }
	}
	for _, c := range []struct {
		name     string
		declared []string
		standing func(string) bool
		want     []string
	}{
		{
			name:     "a column declaring nothing with a card standing ready",
			standing: standingIn(contract.StateReady),
		},
		{
			name:     "a column declaring nothing with a card standing active",
			standing: standingIn(contract.StateActive),
		},
		{
			name:     "a column declaring nothing with a card standing blocked",
			standing: standingIn(contract.StateBlocked),
			want:     []string{contract.StateBlocked},
		},
		{
			name:     "a column declaring ready with a card standing ready",
			declared: []string{contract.StateReady},
			standing: standingIn(contract.StateReady),
			want:     []string{contract.StateReady},
		},
		{
			name:     "a column declaring ready with nothing standing anywhere",
			declared: []string{contract.StateReady},
			standing: standingIn(),
			want:     []string{contract.StateReady},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := StatesDrawn(c.declared, c.standing)
			if strings.Join(got, ",") != strings.Join(c.want, ",") {
				t.Errorf("the rule heads %v and %s heads %v", got, c.name, c.want)
			}
		})
	}
}

// TestStatesShownKeepsTheCardsAHeadingWasRemovedFrom asserts that narrowing the
// headings did not narrow what the tree finds. A column declaring nothing with a
// card standing ready heads nothing and still shows ready, which is the state
// whose cards the caller attaches inline.
func TestStatesShownKeepsTheCardsAHeadingWasRemovedFrom(t *testing.T) {
	standing := func(state string) bool { return state == contract.StateReady }
	if drawn := StatesDrawn(nil, standing); len(drawn) != 0 {
		t.Errorf("the rule heads %v at a column declaring nothing", drawn)
	}
	shown := StatesShown(nil, standing)
	want := []string{contract.StateReady}
	if strings.Join(shown, ",") != strings.Join(want, ",") {
		t.Errorf("the rule shows %v and the card standing there carries %v", shown, want)
	}
}

// TestAQueueColumnHoldingOnlyReadyCardsHeadsNothing is dinah-329's own shape,
// reproduced from the operator's sidebar: two cards standing at a column where
// nobody takes work up, nothing blocked, the default chain, no query and no
// other axis in play. That column heads no state at all and draws both cards as
// bare leaves of itself.
//
// The test above covers the same rule with a blocked card present, which leaves
// one heading standing. This one leaves none, and an implementation that headed
// a state whenever it was the only one a column held would pass that test and
// redden here.
func TestAQueueColumnHoldingOnlyReadyCardsHeadsNothing(t *testing.T) {
	h := newBufferHarness(t)
	first := h.add("first in the queue")
	h.at(first, bufferQueue)
	second := h.add("second in the queue")
	h.at(second, bufferQueue)

	built := treeOf(t, h, "", nil, LevelCards)
	group := groupAt(t, built.Root, bufferQueueSlug)
	if got := groupValues(group); len(got) != 0 {
		t.Errorf("the waiting column heads the state groups %v and every card standing there is ready", got)
	}
	if group.Count != 2 {
		t.Errorf("the waiting column counts %d cards and holds 2", group.Count)
	}
	want := []string{first, second}
	if inline := leafChildRefs(group); strings.Join(inline, ",") != strings.Join(want, ",") {
		t.Errorf("the waiting column attaches %v directly and the cards standing there are %v", inline, want)
	}
	if len(group.Children) != 2 {
		t.Errorf("the waiting column draws %d children and holds two cards and no group", len(group.Children))
	}
}
