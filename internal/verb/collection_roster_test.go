package verb

import (
	"os"
	"strings"
	"testing"
)

// TestTheReferenceTakingRosterIsSeventeen derives the roster at the commit
// under test rather than reading it off a card, because three separate
// statements of this number, in a card's own framing and in its parent's
// prose, disagreed with the tables.
//
// It cannot pass vacuously. An empty parse yields nothing rather than
// seventeen, and the two declarations that carry the fact are compared against
// each other as well as against the named set, so a command declaring the
// guide on one of them and not the other is left out and the count fails.
func TestTheReferenceTakingRosterIsSeventeen(t *testing.T) {
	roster := ReferenceTakingCommands()
	if len(roster) != 17 {
		t.Fatalf("the roster holds %d commands and it is seventeen: %s", len(roster), strings.Join(roster, " "))
	}
	want := []string{
		"archive", "attach", "attachments", "cite", "contents", "delete", "edit",
		"fail", "get", "instructions", "path", "rename", "reopen", "resolve",
		"set", "show", "verify",
	}
	if strings.Join(roster, " ") != strings.Join(want, " ") {
		t.Errorf("the roster is\n  %s\nand the set the card names is\n  %s", strings.Join(roster, " "), strings.Join(want, " "))
	}

	// The two declarations are counted separately as well, because the roster
	// above returns a command only when they agree, so a disagreement would
	// otherwise show up as a shorter list with no reason attached.
	byGuide, byParam := 0, 0
	for _, topics := range guides {
		for _, topic := range topics {
			if topic == referencesGuide {
				byGuide++
			}
		}
	}
	for _, list := range params {
		for _, param := range list {
			if param.Guide == referencesGuide {
				byParam++
				break
			}
		}
	}
	if byGuide != 17 || byParam != 17 {
		t.Errorf("the guide table names %d commands and the parameter table names %d, and both are seventeen", byGuide, byParam)
	}
	t.Logf("the roster derived at this commit holds %d commands", len(roster))
}

// TestTheDocCommentsThisCardFalsifiesWereCorrected reads the three shipped doc
// comments this card's new payload value makes false.
//
// All three are field or type comments on published JSON payloads, so they are
// the contract a machine caller reads, and nothing else in the tree holds
// them: the documentation guards cover guide blocks, prose figures and the
// quick start rather than Go doc comments. The run reports a verdict per
// comment rather than a boolean, so a diff that corrected none reports three
// survivals rather than passing quietly.
func TestTheDocCommentsThisCardFalsifiesWereCorrected(t *testing.T) {
	stale := []struct {
		file     string
		what     string
		survived string
		carries  string
	}{
		{
			file:     "tree.go",
			what:     "TreeNode.Kind",
			survived: "// attachment, a dotted extension kind, or group. Every value but group",
			carries:  "KindCollection",
		},
		{
			file:     "read.go",
			what:     "AttachmentListing.Kind",
			survived: "// Kind is the entity's kind, as the containment grammar spells it.",
			carries:  "verb.KindCollection where the reference named a whole collection",
		},
		{
			file:     "read.go",
			what:     "AttachmentListing.Ref",
			survived: "// Ref is what a person types to reach the entity the attachments hang\n\t// from. The workbench is written `workbench`, which is the spelling the\n\t// containment tree's own root row already prints for it, and everything\n\t// else carries the reference the resolver composed.",
			carries:  "a collection reference",
		},
	}
	if len(stale) != 3 {
		t.Fatalf("the subject set holds %d comments and this card falsifies three", len(stale))
	}
	survivals := 0
	for _, comment := range stale {
		source, err := os.ReadFile(comment.file)
		if err != nil {
			t.Fatalf("read %s: %v", comment.file, err)
		}
		text := string(source)
		if strings.Contains(text, comment.survived) {
			survivals++
			t.Errorf("%s's comment on %s still claims what this card falsifies", comment.file, comment.what)
			continue
		}
		if !strings.Contains(text, comment.carries) {
			t.Errorf("%s's comment on %s no longer carries the stale claim but says nothing about a collection either", comment.file, comment.what)
		}
	}
	t.Logf("three comments read, %d stale claims survived", survivals)
}

// TestTreeNodeRefSaysWhatACollectionRootCarries reads the fourth shipped doc
// comment this card falsifies, which the sweep behind
// TestTheDocCommentsThisCardFalsifiesWereCorrected missed.
//
// That sweep found TreeNode.Kind and stopped, and TreeNode.Ref sits two fields
// below it on the same struct. Its sentence says the value is what a person
// types to reach the node on show, path, or edit, and this card makes edit
// refuse a collection while contents grows a walk whose root node carries a
// collection reference. So the one comment on the struct that names the three
// commands became false in the same diff that corrected the one above it.
//
// It is guarded here rather than added to that sweep's subject set, because
// that set is three by assertion and the criterion behind it names its three
// comments one by one.
func TestTreeNodeRefSaysWhatACollectionRootCarries(t *testing.T) {
	source, err := os.ReadFile("tree.go")
	if err != nil {
		t.Fatalf("read tree.go: %v", err)
	}
	text := string(source)
	// The fragment sits whole on one source line at b825059, so the grep is
	// not defeated by the wrap the sentence it belongs to takes.
	stale := "// It is absent on a group node, which nothing addresses.\n"
	if strings.Contains(text, stale) {
		t.Fatalf("tree.go's comment on TreeNode.Ref still ends where it ended before edit refused a collection")
	}
	if !strings.Contains(text, "edit refuses") {
		t.Errorf("tree.go's comment on TreeNode.Ref no longer ends there but says nothing about what edit does with a collection reference")
	}
	t.Logf("tree.go's comment on TreeNode.Ref covers the collection root")
}
