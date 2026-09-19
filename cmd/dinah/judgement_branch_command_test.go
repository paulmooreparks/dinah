package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/verb"
)

// judgementBench is a card whose checklist is interleaved rather than sorted,
// so an implementation that grouped by the order it met kinds in would draw
// the branches in the order Criteria, Decisions, Questions and fail every
// order assertion below. It also carries a comment and an attachment, which
// are what hold the branches to the mount positions the checklist already
// stood between.
func judgementBench(t *testing.T) string {
	t.Helper()
	root := newBench(t)
	mustRun(t, root, "add", "a card whose judgements are branched")
	mustRun(t, root, "add", "a card carrying no judgement at all")
	mustRun(t, root, "comment", "fx-1", "the first thought")
	mustRun(t, root, "file", "fx-1", "acceptance_criterion", "the endpoint answers 404")
	mustRun(t, root, "file", "fx-1", "decision", "the exporter is shared")
	mustRun(t, root, "file", "fx-1", "open_question", "does the deadline move?")
	mustRun(t, root, "file", "fx-1", "acceptance_criterion", "the export is stable")
	mustRun(t, root, "file", "fx-1", "open_question", "which region?")
	mustRun(t, root, "file", "fx-1", "decision", "the deadline holds")
	source := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(source, []byte("some bytes"), 0o644); err != nil {
		t.Fatalf("write the attachment source: %v", err)
	}
	mustRun(t, root, "attach", "fx-1", source, "--description", "the first file")
	return root
}

// TestACardPublishesItsJudgementsAsThreeBranches is the card's own shape, read
// off the machine surface: three branches in declaration order, standing where
// the checklist mount stood, with the comment before them and the attachment
// after.
func TestACardPublishesItsJudgementsAsThreeBranches(t *testing.T) {
	root := judgementBench(t)
	tree := treeOf(t, root, "fx-1")

	var drawn []string
	for _, child := range tree.Root.Children {
		drawn = append(drawn, child.Kind+" "+child.Ref)
	}
	want := []string{
		"comment fx-1/comments/1",
		"collection fx-1/questions",
		"collection fx-1/criteria",
		"collection fx-1/decisions",
		"attachment fx-1/attachments/1",
	}
	if strings.Join(drawn, "\n") != strings.Join(want, "\n") {
		t.Fatalf("the card draws\n  %s\nwant\n  %s", strings.Join(drawn, " | "), strings.Join(want, " | "))
	}

	for _, expected := range []struct {
		ref      string
		narrow   string
		members  int
		children []string
	}{
		{ref: "fx-1/questions", narrow: "open_question", members: 2,
			children: []string{"fx-1/questions/1", "fx-1/questions/2"}},
		{ref: "fx-1/criteria", narrow: "acceptance_criterion", members: 2,
			children: []string{"fx-1/criteria/1", "fx-1/criteria/2"}},
		{ref: "fx-1/decisions", narrow: "decision", members: 2,
			children: []string{"fx-1/decisions/1", "fx-1/decisions/2"}},
	} {
		branch := childOf(t, tree.Root, expected.ref)
		if branch.MemberKind != "item" {
			t.Errorf("%s holds member kind %q, want item", branch.Ref, branch.MemberKind)
		}
		if branch.Narrow != expected.narrow {
			t.Errorf("%s narrows by %q, want %q", branch.Ref, branch.Narrow, expected.narrow)
		}
		if branch.MemberCount == nil {
			t.Fatalf("%s publishes no member count", branch.Ref)
		}
		if *branch.MemberCount != expected.members {
			t.Errorf("%s publishes the member count %d, want %d", branch.Ref, *branch.MemberCount, expected.members)
		}
		if branch.ID != "" {
			t.Errorf("%s carries the identifier %q and a branch has none", branch.Ref, branch.ID)
		}
		var refs []string
		for _, member := range branch.Children {
			refs = append(refs, member.Ref)
		}
		if strings.Join(refs, ",") != strings.Join(expected.children, ",") {
			t.Errorf("%s draws %v, want %v", branch.Ref, refs, expected.children)
		}
	}
}

// TestACardWithNoJudgementsDrawsNoBranch holds the other half of the rule that
// empty branches are omitted: a card carrying none of the three draws none of
// them rather than three empty rows.
func TestACardWithNoJudgementsDrawsNoBranch(t *testing.T) {
	root := judgementBench(t)
	tree := treeOf(t, root, "fx-2")
	for _, child := range tree.Root.Children {
		if child.Kind == verb.KindCollection {
			t.Errorf("a card with no judgements draws the branch %s", child.Ref)
		}
	}
}

// TestAnEntityNodeCarriesNoneOfTheCollectionMembers holds the optionality the
// three new members are declared with: they are a collection's business, and
// an entity node that published them would be telling a reader something about
// itself that is not true.
func TestAnEntityNodeCarriesNoneOfTheCollectionMembers(t *testing.T) {
	root := judgementBench(t)
	payload := mustRun(t, root, "list", "fx-1", "--depth", "all", "--json").out
	var raw struct {
		Root json.RawMessage `json:"root"`
	}
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		t.Fatalf("the tree will not parse: %v", err)
	}
	var walk func(node json.RawMessage)
	walk = func(node json.RawMessage) {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(node, &fields); err != nil {
			t.Fatalf("a node will not parse: %v", err)
		}
		kind := ""
		if err := json.Unmarshal(fields["kind"], &kind); err != nil {
			t.Fatalf("a node's kind will not parse: %v", err)
		}
		for _, member := range []string{"member_kind", "narrow", "member_count"} {
			_, present := fields[member]
			if kind != verb.KindCollection && present {
				t.Errorf("a %s node publishes %s, which belongs to a collection", kind, member)
			}
			if kind == verb.KindCollection && member != "narrow" && !present {
				t.Errorf("a collection node publishes no %s", member)
			}
		}
		if children, ok := fields["children"]; ok {
			var members []json.RawMessage
			if err := json.Unmarshal(children, &members); err != nil {
				t.Fatalf("a node's children will not parse: %v", err)
			}
			for _, member := range members {
				walk(member)
			}
		}
	}
	walk(raw.Root)
}

// TestANarrowedRootDoesNotWrapItselfInASecondBranch holds the direct-root rule
// for all four spellings a checklist collection answers to, including the
// short one, and for the unnarrowed root that groups what it holds.
func TestANarrowedRootDoesNotWrapItselfInASecondBranch(t *testing.T) {
	root := judgementBench(t)

	for _, spelling := range []struct {
		ref     string
		narrow  string
		members int
		first   string
	}{
		{ref: "fx-1/questions", narrow: "open_question", members: 2, first: "fx-1/questions/1"},
		{ref: "fx-1/criteria", narrow: "acceptance_criterion", members: 2, first: "fx-1/criteria/1"},
		{ref: "fx-1/decisions", narrow: "decision", members: 2, first: "fx-1/decisions/1"},
		{ref: "fx-1/oq", narrow: "open_question", members: 2, first: "fx-1/questions/1"},
	} {
		tree := treeOf(t, root, spelling.ref)
		if tree.Root.Ref != spelling.ref {
			t.Errorf("the root of %s echoes %q, and a root echoes what the caller typed", spelling.ref, tree.Root.Ref)
		}
		if tree.Root.Narrow != spelling.narrow {
			t.Errorf("%s narrows by %q, want %q", spelling.ref, tree.Root.Narrow, spelling.narrow)
		}
		if tree.Root.MemberCount == nil || *tree.Root.MemberCount != spelling.members {
			t.Errorf("%s publishes the member count %v, want %d", spelling.ref, tree.Root.MemberCount, spelling.members)
		}
		if len(tree.Root.Children) != spelling.members {
			t.Fatalf("%s draws %d children, want %d", spelling.ref, len(tree.Root.Children), spelling.members)
		}
		for _, child := range tree.Root.Children {
			if child.Kind == verb.KindCollection {
				t.Errorf("%s wrapped itself in the second branch %s", spelling.ref, child.Ref)
			}
		}
		// The children carry canonical long references whatever the root was
		// typed as, which is what the short spelling is here to prove.
		if tree.Root.Children[0].Ref != spelling.first {
			t.Errorf("%s draws its first member as %s, want the canonical %s",
				spelling.ref, tree.Root.Children[0].Ref, spelling.first)
		}
	}

	// The unnarrowed root groups the members it holds, which is the same rule
	// the card applies one rung up.
	whole := treeOf(t, root, "fx-1/checklist")
	var drawn []string
	for _, child := range whole.Root.Children {
		drawn = append(drawn, child.Kind+" "+child.Ref)
	}
	want := []string{
		"collection fx-1/questions",
		"collection fx-1/criteria",
		"collection fx-1/decisions",
	}
	if strings.Join(drawn, "\n") != strings.Join(want, "\n") {
		t.Errorf("the checklist root draws\n  %s\nwant\n  %s", strings.Join(drawn, " | "), strings.Join(want, " | "))
	}
	if whole.Root.MemberCount == nil || *whole.Root.MemberCount != 6 {
		t.Errorf("the checklist root publishes the member count %v and holds 6 items", whole.Root.MemberCount)
	}
}

// TestADirectlyNamedEmptyCollectionAnswersAsAnEmptyRoot holds what a reference
// that is valid today goes on meaning. A card carrying no question still has
// a questions collection to name, and naming it succeeds with nothing in it
// rather than refusing.
func TestADirectlyNamedEmptyCollectionAnswersAsAnEmptyRoot(t *testing.T) {
	root := judgementBench(t)
	tree := treeOf(t, root, "fx-2/questions")
	if tree.Root.Ref != "fx-2/questions" {
		t.Errorf("the empty root echoes %q, want fx-2/questions", tree.Root.Ref)
	}
	if tree.Root.MemberCount == nil {
		t.Fatal("the empty root publishes no member count, and zero is what it has to say")
	}
	if *tree.Root.MemberCount != 0 {
		t.Errorf("the empty root publishes the member count %d, want 0", *tree.Root.MemberCount)
	}
	if tree.Root.Count != 0 {
		t.Errorf("the empty root counts %d entities below it, want 0", tree.Root.Count)
	}
	if len(tree.Root.Children) != 0 {
		t.Errorf("the empty root draws %d children, want none", len(tree.Root.Children))
	}
}

// TestTheHumanTranscriptDrawsABranchRowPerJudgementKind is the printed surface
// the machine one above is the other half of. The rows are asserted by
// reference and entity rather than by their exact spacing, which the table
// aligns on the widest cell and is not this card's business.
func TestTheHumanTranscriptDrawsABranchRowPerJudgementKind(t *testing.T) {
	root := judgementBench(t)
	printed := mustRun(t, root, "list", "fx-1", "--depth", "all").out

	for _, row := range []struct {
		reference string
		entity    string
	}{
		{reference: "fx-1/questions", entity: "collection"},
		{reference: "fx-1/criteria", entity: "collection"},
		{reference: "fx-1/decisions", entity: "collection"},
		{reference: "fx-1/questions/1", entity: "item"},
		{reference: "fx-1/decisions/2", entity: "item"},
	} {
		if !containsRow(printed, row.reference, row.entity) {
			t.Errorf("the transcript draws no %s row for %s:\n%s", row.entity, row.reference, printed)
		}
	}
	// A branch row's count is the entities below it, which for a branch of two
	// items carrying nothing of their own is two.
	if !containsRow(printed, "fx-1/questions", "collection") {
		t.Errorf("the transcript draws no questions branch:\n%s", printed)
	}
}

// childOf is one named child of a node, failed for rather than skipped when it
// is not there, so a missing branch reports as itself.
func childOf(t *testing.T, node verb.TreeNode, ref string) verb.TreeNode {
	t.Helper()
	for _, child := range node.Children {
		if child.Ref == ref {
			return child
		}
	}
	t.Fatalf("%s draws no child %s", node.Ref, ref)
	return verb.TreeNode{}
}

// containsRow reports whether a printed table draws a row naming one reference
// against one entity word, whatever the column widths came out at.
func containsRow(printed, reference, entity string) bool {
	for _, line := range strings.Split(printed, "\n") {
		fields := strings.Fields(line)
		for i, field := range fields {
			if field == reference && i+1 < len(fields) && fields[i+1] == entity {
				return true
			}
		}
	}
	return false
}
