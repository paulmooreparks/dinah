package main

import (
	"strings"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/msg"
)

// TestTheTerminalDrawsTheIndexAndTeachesTheRightRecovery asserts the two
// halves of this change a person meets: the comments block draws a subject
// and a size where it used to draw the whole comment, and the sentence under
// the announcement names the lever that would actually serve the rest.
//
// The second half is the one worth the fixture. A caller who wrote
// `--unresolved --fields checklist.full` has already named every member the
// announcement could ask them for, so the ordinary sentence would send them
// to write the call they have just written. The two sentences are compared
// against each other as well as asserted present, because a build printing
// both would satisfy either assertion alone.
func TestTheTerminalDrawsTheIndexAndTeachesTheRightRecovery(t *testing.T) {
	root := newBench(t)
	card := addCard(t, root, "A card worth reading twice")
	if got := runCLI(t, root, "comment", card, "## TRIAGE\n\nthe body nobody asked for"); got.code != 0 {
		t.Fatalf("comment: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "file", card, "open_question", "Which vendor do we cite?"); got.code != 0 {
		t.Fatalf("file the question: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "file", card, "decision", "Whose contract the numbers come from."); got.code != 0 {
		t.Fatalf("file the decision: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "resolve", card+"/decisions/1", "the operator settled it"); got.code != 0 {
		t.Fatalf("resolve: %d %s", got.code, got.errw)
	}

	catalog := msg.For(msg.Base)
	index := runCLI(t, root, "show", card)
	if index.code != 0 {
		t.Fatalf("show: %d %s", index.code, index.errw)
	}
	for _, heading := range []string{catalog.T("column.comments.subject"), catalog.T("column.comments.size")} {
		if !strings.Contains(index.out, heading) {
			t.Errorf("the comments block draws no %q column: %q", heading, index.out)
		}
	}
	if !strings.Contains(index.out, "TRIAGE") {
		t.Errorf("the comments block draws no subject: %q", index.out)
	}
	if strings.Contains(index.out, "the body nobody asked for") {
		t.Errorf("the comments block drew a body: %q", index.out)
	}
	if strings.Contains(index.out, "the operator settled it") {
		t.Errorf("the checklist block drew a resolution note it was not asked for: %q", index.out)
	}

	// The note is the one thing checklist.full adds to that block, and it is
	// drawn under the row it belongs to.
	full := runCLI(t, root, "show", card, "--fields", "checklist.full")
	if full.code != 0 {
		t.Fatalf("show --fields checklist.full: %d %s", full.code, full.errw)
	}
	if !strings.Contains(full.out, "the operator settled it") {
		t.Errorf("checklist.full drew no resolution note: %q", full.out)
	}

	// The filter narrows the block, and the sentence under the announcement
	// changes with it.
	ordinary := catalog.T("show.reread", "reread", card)
	refilter := catalog.T("show.refilter", "reread", card, "flags", "--unresolved")
	filtered := runCLI(t, root, "show", card, "--unresolved", "--fields", "checklist.full")
	if filtered.code != 0 {
		t.Fatalf("show --unresolved --fields checklist.full: %d %s", filtered.code, filtered.errw)
	}
	if strings.Contains(filtered.out, "Whose contract the numbers come from") {
		t.Errorf("the filter carried an item that lifts a column hold: %q", filtered.out)
	}
	if !strings.Contains(filtered.out, "Which vendor do we cite?") {
		t.Errorf("the filter dropped the item that still holds the card: %q", filtered.out)
	}
	if !strings.Contains(filtered.out, refilter) {
		t.Errorf("wanted the sentence naming the flag to drop, %q, got %q", refilter, filtered.out)
	}
	if strings.Contains(filtered.out, ordinary) {
		t.Errorf("the filtered answer told the reader to name a member they had already named: %q", filtered.out)
	}
	// The control, which is the same card read without the filter: there the
	// ordinary sentence is the right one and the other is absent.
	unfiltered := runCLI(t, root, "show", card, "--fields", "card,checklist")
	if unfiltered.code != 0 {
		t.Fatalf("show --fields card,checklist: %d %s", unfiltered.code, unfiltered.errw)
	}
	if !strings.Contains(unfiltered.out, ordinary) {
		t.Errorf("wanted the ordinary recovery sentence, %q, got %q", ordinary, unfiltered.out)
	}
	if strings.Contains(unfiltered.out, refilter) {
		t.Errorf("an answer no filter narrowed told the reader to drop a flag: %q", unfiltered.out)
	}

	// A filter the field list gives no member to shape is refused rather
	// than accepted and dropped, and the accepting case sits beside it.
	refused := runCLI(t, root, "show", card, "--fields", "card,body", "--since", "1")
	if refused.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("a filter without its member exited %d: %s%s", refused.code, refused.out, refused.errw)
	}
	if name := refusalNameOf(refused.errw); name != contract.Usage {
		t.Errorf("the refusal is %s, wanted %s", name, contract.Usage)
	}
	accepted := runCLI(t, root, "show", card, "--fields", "card,comments", "--since", "1")
	if accepted.code != 0 {
		t.Fatalf("a filter naming its member exited %d: %s", accepted.code, accepted.errw)
	}
	if !strings.Contains(accepted.out, card+"/comments/1") {
		t.Errorf("the accepted call carried no comments block: %q", accepted.out)
	}
}

// TestAStackedChecklistDrawsItsResolutionNote asserts the note survives the
// layout a narrow window forces.
//
// The checklist block declares labelInTheStack, so a window too narrow for
// its columns draws each row as labelled lines instead of a table row. Before
// this card no call site stacked a row carrying a note, because the checklist
// drew none and the comments block is not a stacking table, so the line that
// draws one had no reader. `checklist.full` is the first surface that asks
// for both at once, and a note dropped in that layout would be invisible to
// every assertion in the test above, each of which reads a wide window.
func TestAStackedChecklistDrawsItsResolutionNote(t *testing.T) {
	t.Setenv("COLUMNS", "40")
	root := newBench(t)
	card := addCard(t, root, "A card read through a narrow window")
	if got := runCLI(t, root, "file", card, "decision", "Whose contract the numbers come from."); got.code != 0 {
		t.Fatalf("file the decision: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "resolve", card+"/decisions/1", "the operator settled it"); got.code != 0 {
		t.Fatalf("resolve: %d %s", got.code, got.errw)
	}

	full := runCLI(t, root, "show", card, "--fields", "card,checklist.full")
	if full.code != 0 {
		t.Fatalf("show --fields card,checklist.full: %d %s", full.code, full.errw)
	}
	// A stacked block draws no rule under a heading row, because it draws no
	// heading row at all, so the rule's absence is what says the window
	// forced the layout this test is about.
	if strings.Contains(full.out, "---") {
		t.Fatalf("the window was wide enough to draw a table, so the stacked layout went untested: %q", full.out)
	}
	if !strings.Contains(full.out, "the operator settled it") {
		t.Errorf("the stacked checklist drew no resolution note: %q", full.out)
	}
}
