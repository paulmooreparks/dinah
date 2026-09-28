package main

import (
	"strings"
	"testing"

	"dinah/internal/msg"
)

// TestShowBriefDrawsTheHandoffBlock asserts dinah-648's rendering half at the
// terminal: `dinah show <card> --brief` draws the handoff under its own label
// with the comment's body, leaves the comment index out, and names the
// comments as withheld with a recovery that drops --brief. The card carries a
// comment written before its move and one written after, because the block
// proves nothing unless the answer tells the two apart.
func TestShowBriefDrawsTheHandoffBlock(t *testing.T) {
	root := newBench(t)
	card := addCard(t, root, "A card a station opens on")
	const handoff = "## HANDOFF\n\nRead this before anything else."
	if got := runCLI(t, root, "comment", card, handoff); got.code != 0 {
		t.Fatalf("comment %s: %d %s", card, got.code, got.errw)
	}
	carryToDoing(t, root, card)
	if got := runCLI(t, root, "comment", card, "A remark the stay under way wrote."); got.code != 0 {
		t.Fatalf("second comment on %s: %d %s", card, got.code, got.errw)
	}

	brief := runCLI(t, root, "show", card, "--brief")
	if brief.code != 0 {
		t.Fatalf("show %s --brief: %d %s", card, brief.code, brief.errw)
	}
	label := msg.For(msg.Base).T("show.handoff")
	if !strings.Contains(brief.out, label) {
		t.Errorf("the brief draws no %q block: %q", label, brief.out)
	}
	if !strings.Contains(brief.out, "Read this before anything else.") {
		t.Errorf("the handoff block carries no body: %q", brief.out)
	}
	if strings.Contains(brief.out, "the stay under way") {
		t.Errorf("the brief drew a comment written after the latest move: %q", brief.out)
	}
	if strings.Contains(brief.out, msg.For(msg.Base).T("show.comments")) {
		t.Errorf("the brief drew the comment index: %q", brief.out)
	}
	withheld := msg.For(msg.Base).T("show.withheld", "members", "comments, path")
	if !strings.Contains(brief.out, withheld) {
		t.Errorf("wanted the line %q, got %q", withheld, brief.out)
	}

	// The control. A field list naming the handoff on a card that never
	// moved draws no block, and the announcement names the comments rather
	// than a handoff the card does not hold.
	still := addCard(t, root, "A card that never moved")
	if got := runCLI(t, root, "comment", still, "Written where the card was filed."); got.code != 0 {
		t.Fatalf("comment %s: %d %s", still, got.code, got.errw)
	}
	none := runCLI(t, root, "show", still, "--fields", "handoff")
	if none.code != 0 {
		t.Fatalf("show %s --fields handoff: %d %s", still, none.code, none.errw)
	}
	if strings.Contains(none.out, label) {
		t.Errorf("a card that never moved drew a handoff block: %q", none.out)
	}
	if !strings.Contains(none.out, msg.For(msg.Base).T("show.withheld", "members", "card, comments, path")) {
		t.Errorf("the announcement on the unmoved card is wrong: %q", none.out)
	}
}
