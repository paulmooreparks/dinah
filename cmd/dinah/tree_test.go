package main

import (
	"encoding/json"
	"strings"
	"testing"

	"dinah/internal/msg"
	"dinah/internal/verb"
)

// treeJSON runs a tree command under --json and reads the object back.
func treeJSON(t *testing.T, root string, argv ...string) verb.Tree {
	t.Helper()
	got := runCLI(t, root, append(argv, "--json")...)
	if got.code != 0 {
		t.Fatalf("%v: exit %d\n%s", argv, got.code, got.errw)
	}
	var built verb.Tree
	if err := json.Unmarshal([]byte(got.out), &built); err != nil {
		t.Fatalf("%v: %v\n%s", argv, err, got.out)
	}
	return built
}

// TestTheTreeDrawsNoRowForItsRoot asserts the rendering the operator ruled: the
// root's title, reference and total print in the sentence above the table, the
// table starts at the root's children, and the machine form still carries the
// root as a node.
func TestTheTreeDrawsNoRowForItsRoot(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "a card of the intake")
	runCLI(t, root, "add", "a second card")

	got := runCLI(t, root, "tree")
	if got.code != 0 {
		t.Fatalf("tree: exit %d\n%s", got.code, got.errw)
	}
	lines := strings.Split(strings.TrimRight(got.out, "\n"), "\n")
	header := msg.For(msg.Base).T("tree.header", "title", "workbench", "ref", "fx", "count", "2")
	if lines[0] != header {
		t.Errorf("the sentence above the tree reads %q, want %q", lines[0], header)
	}
	for _, line := range lines[1:] {
		if strings.Contains(line, "  fx  ") {
			t.Errorf("the table draws a row for the root:\n%q", line)
		}
	}
	built := treeJSON(t, root, "tree")
	if built.Root.Ref != "fx" || built.Root.Count != 2 {
		t.Errorf("the machine form carries the root as %q counting %d, want fx counting 2", built.Root.Ref, built.Root.Count)
	}
	if built.Producer != verb.ProducerGrouped || built.Subject != verb.SubjectCard {
		t.Errorf("the tree reports the producer %q over the subject %q", built.Producer, built.Subject)
	}
}

// TestAFilteredHeaderNamesBothNumbers asserts that the sentence above a
// filtered tree says what the workbench holds and how much of it matched, in
// that order.
func TestAFilteredHeaderNamesBothNumbers(t *testing.T) {
	root := newBench(t)
	for range 3 {
		runCLI(t, root, "add", "a card of the intake")
	}
	runCLI(t, root, "claim", "fx-1")

	got := runCLI(t, root, "tree", "substate:ready")
	if got.code != 0 {
		t.Fatalf("tree: exit %d\n%s", got.code, got.errw)
	}
	want := msg.For(msg.Base).T("tree.header.filtered", "title", "workbench", "ref", "fx", "held", "3", "matched", "2")
	if first := strings.SplitN(got.out, "\n", 2)[0]; first != want {
		t.Errorf("the sentence above the filtered tree reads %q, want %q", first, want)
	}
}

// TestTheNotShownCellPrintsTheChildrenTheDepthCutOff asserts the member the
// depth sentence carries.
//
// The fixture is a card carrying five entities none of which contains
// anything, so the count of the children the depth cut off is five while the
// subjects those children hold number zero, and a renderer reading the other
// member prints zero. The walk starts at the workbench so the card is a row
// rather than the root, since the root draws no row. The rendered number is
// asserted against the machine form of the same call, since under the grouped
// producer the two members are equal at every boundary and no grouped fixture
// can tell them apart.
func TestTheNotShownCellPrintsTheChildrenTheDepthCutOff(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "a card with things below it")
	for _, note := range []string{"the first note", "the second note", "the third note", "the fourth note", "the fifth note"} {
		runCLI(t, root, "comment", "fx-1", note)
	}

	built := treeJSON(t, root, "contents", "workbench", "--depth", "cards")
	var card *verb.TreeNode
	for i := range built.Root.Children {
		if built.Root.Children[i].Ref == "fx-1" {
			card = &built.Root.Children[i]
		}
	}
	if card == nil || card.Hidden == nil {
		t.Fatalf("the card draws no row reporting what the depth cut off: %+v", built.Root.Children)
	}
	if card.Hidden.Children != 5 || card.Hidden.Subjects != 0 {
		t.Fatalf("the card reports %d children and %d subjects, want 5 and 0",
			card.Hidden.Children, card.Hidden.Subjects)
	}
	got := runCLI(t, root, "contents", "workbench", "--depth", "cards")
	if got.code != 0 {
		t.Fatalf("contents: exit %d\n%s", got.code, got.errw)
	}
	sentence := msg.For(msg.Base).T("tree.hidden.depth", "count", "5")
	if !strings.Contains(got.out, sentence) {
		t.Errorf("the rendering does not carry %q:\n%s", sentence, got.out)
	}
	wrong := msg.For(msg.Base).T("tree.hidden.depth", "count", "0")
	if strings.Contains(got.out, wrong) {
		t.Errorf("the rendering carries %q, which is the other member:\n%s", wrong, got.out)
	}
}

// TestAnEntityWithNothingBelowItPrintsOneSentence asserts that a containment
// root that is itself a leaf prints its own sentence, draws no table and
// succeeds, and that the machine form carries a root counting nothing with no
// children and no account of anything held back.
func TestAnEntityWithNothingBelowItPrintsOneSentence(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "a card with nothing below it")

	got := runCLI(t, root, "contents", "fx-1")
	if got.code != 0 {
		t.Fatalf("contents: exit %d\n%s", got.code, got.errw)
	}
	lines := strings.Split(strings.TrimRight(got.out, "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("the empty answer draws %d lines and it is one sentence:\n%s", len(lines), got.out)
	}
	want := msg.For(msg.Base).T("contents.empty", "title", "a card with nothing below it", "ref", "fx-1")
	if lines[0] != want {
		t.Errorf("the empty answer reads %q, want %q", lines[0], want)
	}
	built := treeJSON(t, root, "contents", "fx-1")
	if built.Root.Count != 0 || len(built.Root.Children) != 0 || built.Root.Hidden != nil {
		t.Errorf("the machine form counts %d with %d children and reports %v", built.Root.Count, len(built.Root.Children), built.Root.Hidden)
	}
}

// TestTheNotShownColumnIsDrawnOnlyWhenARowFillsIt asserts both halves of the
// elision rule as a reader meets them: a tree holding nothing back draws four
// columns, and one holding something back draws the fifth.
func TestTheNotShownColumnIsDrawnOnlyWhenARowFillsIt(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "a card of the intake")

	full := runCLI(t, root, "tree")
	if full.code != 0 {
		t.Fatalf("tree: exit %d\n%s", full.code, full.errw)
	}
	hiddenHeading := msg.For(msg.Base).T("column.tree.hidden")
	if strings.Contains(full.out, hiddenHeading) {
		t.Errorf("a tree that hides nothing draws the %q column:\n%s", hiddenHeading, full.out)
	}
	for _, key := range []string{"column.tree.reference", "column.tree.entity", "column.tree.title", "column.tree.count"} {
		if heading := msg.For(msg.Base).T(key); !strings.Contains(full.out, heading) {
			t.Errorf("the tree draws no %q column:\n%s", heading, full.out)
		}
	}
	bounded := runCLI(t, root, "tree", "--depth", "groups")
	if bounded.code != 0 {
		t.Fatalf("tree at the groups level: exit %d\n%s", bounded.code, bounded.errw)
	}
	if !strings.Contains(bounded.out, hiddenHeading) {
		t.Errorf("a tree holding its cards back draws no %q column:\n%s", hiddenHeading, bounded.out)
	}
}

// TestThreeNamesCoverTheFourBadChains asserts that a reader meets a different
// refusal for a word this tool does not group on, for a legal axis named
// twice, and for a chain longer than the command nests along, that all four
// exit 2, and that only the first lists the axes. One name over all four would
// print a sentence calling a legal axis not an axis and then listing it.
func TestThreeNamesCoverTheFourBadChains(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "a card of the intake")
	axes := strings.Join(verb.GroupAxes(), ", ")
	cases := []struct {
		chain   string
		refusal string
		lists   bool
	}{
		{chain: "priority", refusal: "dinah.unknown-axis", lists: true},
		{chain: "at", refusal: "dinah.unknown-axis", lists: true},
		{chain: "state,state", refusal: "dinah.repeated-axis"},
		{chain: "state,substate,holder,actor,event", refusal: "dinah.chain-too-long"},
	}
	for _, c := range cases {
		got := runCLI(t, root, "tree", "--group-by", c.chain)
		if got.code != 2 {
			t.Errorf("--group-by %s exits %d, want 2", c.chain, got.code)
		}
		if !strings.HasPrefix(got.errw, c.refusal+" ") {
			t.Errorf("--group-by %s refuses with %q, want %s", c.chain, firstLineOf(got.errw), c.refusal)
		}
		if listed := strings.Contains(got.errw, axes); listed != c.lists {
			t.Errorf("--group-by %s lists the nine axes: %v, want %v\n%s", c.chain, listed, c.lists, got.errw)
		}
	}
	depth := runCLI(t, root, "tree", "--depth", "entities")
	if depth.code != 2 || !strings.HasPrefix(depth.errw, "dinah.unknown-depth ") {
		t.Errorf("an unknown depth exits %d with %q", depth.code, firstLineOf(depth.errw))
	}
	if strings.Contains(depth.errw, verb.LevelAll) {
		t.Errorf("the depth refusal lists a level the other command declares:\n%s", depth.errw)
	}
}

// firstLineOf is the first line of a stream, which is the line a refusal
// names itself on.
func firstLineOf(text string) string {
	return strings.SplitN(strings.TrimRight(text, "\n"), "\n", 2)[0]
}

// TestARefusalReadsInHindiWithTheAxesInLatin asserts that a sentence a person
// reads is translated and that the axis names inside it are not, since an axis
// name is what a person types.
func TestARefusalReadsInHindiWithTheAxesInLatin(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "a card of the intake")
	got := runCLI(t, root, "--lang", "hi", "tree", "--group-by", "priority")
	if got.code != 2 {
		t.Fatalf("the refusal exits %d, want 2\n%s", got.code, got.errw)
	}
	if !strings.HasPrefix(got.errw, "dinah.unknown-axis ") {
		t.Fatalf("the refusal names itself %q", firstLineOf(got.errw))
	}
	if !strings.Contains(got.errw, msg.For("hi").T("refusal.dinah.unknown-axis", "detail", "priority", "axes", strings.Join(verb.GroupAxes(), ", "))) {
		t.Errorf("the Hindi refusal does not carry the Hindi sentence:\n%s", got.errw)
	}
	for _, axis := range verb.GroupAxes() {
		if !strings.Contains(got.errw, axis) {
			t.Errorf("the Hindi refusal does not carry the axis %s in Latin script:\n%s", axis, got.errw)
		}
	}
}

// TestEveryKeyOfTheTreeShipsInEveryLocale asserts that no shipped catalog is
// missing a sentence this card added.
func TestEveryKeyOfTheTreeShipsInEveryLocale(t *testing.T) {
	keys := []string{
		"cmd.tree.summary", "cmd.contents.summary",
		"column.tree.reference", "column.tree.entity", "column.tree.title",
		"column.tree.count", "column.tree.hidden",
		"tree.header", "tree.header.filtered", "tree.unset", "tree.empty",
		"tree.hidden.depth", "tree.hidden.filter", "tree.hidden.join",
		"contents.header", "contents.empty",
		"check.tree.1", "check.tree.2", "check.tree.3", "check.tree.4",
		"check.contents.1", "check.contents.2",
		"refusal.dinah.unknown-axis", "refusal.dinah.unknown-axis.next",
		"refusal.dinah.repeated-axis", "refusal.dinah.repeated-axis.next",
		"refusal.dinah.chain-too-long", "refusal.dinah.chain-too-long.next",
		"refusal.dinah.unknown-depth", "refusal.dinah.unknown-depth.next",
	}
	tags := msg.Tags()
	if len(tags) != 8 {
		t.Fatalf("the tool ships %d catalogs and the format declares eight", len(tags))
	}
	for _, tag := range tags {
		catalog := msg.For(tag)
		for _, key := range keys {
			if !catalog.Has(key) {
				t.Errorf("the %s catalog carries no %s", tag, key)
			}
		}
	}
	english, hindi := msg.For(msg.Base), msg.For("hi")
	for _, key := range keys {
		if english.T(key) == hindi.T(key) && !strings.HasPrefix(key, "tree.hidden.join") {
			t.Errorf("%s reads the same in English and Hindi, so one of the two is untranslated", key)
		}
	}
}

// TestTheNoValueGroupCarriesItsOwnLabel asserts that the group holding the
// cards carrying no value on the axis is labelled rather than left blank,
// since a blank cell reads as a rendering fault.
func TestTheNoValueGroupCarriesItsOwnLabel(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "a card somebody holds")
	runCLI(t, root, "add", "a card nobody holds")
	runCLI(t, root, "claim", "fx-1")

	got := runCLI(t, root, "tree", "--group-by", "holder")
	if got.code != 0 {
		t.Fatalf("tree: exit %d\n%s", got.code, got.errw)
	}
	label := msg.For(msg.Base).T("tree.unset")
	if !strings.Contains(got.out, label) {
		t.Errorf("the tree draws no %q group:\n%s", label, got.out)
	}
}

// TestANodeHoldingBackBothKindsJoinsTheTwoSentences asserts that a node the
// depth cut children off and the filter removed cards from below reports both
// in one cell, in the fixed order.
func TestANodeHoldingBackBothKindsJoinsTheTwoSentences(t *testing.T) {
	root := newBench(t)
	for range 3 {
		runCLI(t, root, "add", "a card of the intake")
	}
	runCLI(t, root, "claim", "fx-1")

	got := runCLI(t, root, "tree", "substate:ready", "--group-by", "state", "--depth", "groups")
	if got.code != 0 {
		t.Fatalf("tree: exit %d\n%s", got.code, got.errw)
	}
	catalog := msg.For(msg.Base)
	want := catalog.T("tree.hidden.join",
		"first", catalog.T("tree.hidden.depth", "count", "2"),
		"second", catalog.T("tree.hidden.filter", "count", "1"),
	)
	if !strings.Contains(got.out, want) {
		t.Errorf("the tree does not carry %q:\n%s", want, got.out)
	}
}
