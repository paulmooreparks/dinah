package main

import (
	"strings"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// gluedTail is the tail rowIsGlued renders after the field it is measuring.
// Its own width is subtracted back out, so the marker's spelling decides
// nothing.
const gluedTail = "TAIL"

// tailStartColumn reports the display column a rendered row's tail starts at,
// which is the whole line's width less the tail's own, since formatRow writes
// the tail last and writes nothing after it. Taking the answer from the
// rendered bytes is what lets this measure a line no renderer produced, which
// is how TestRowIsGluedUnitCases proves the detector below can fire at all.
func tailStartColumn(rendered, tail string) int {
	return displayWidth(rendered) - displayWidth(tail)
}

// rowIsGlued renders text through the product's own formatRow rather than
// recomputing what formatRow ought to do, then measures where the tail landed.
// A field formatRow gives its own line is correctly handled by construction
// and is never glued, whatever its length. A field it pads in place is glued
// exactly when the tail did not land on the declared column, which is what a
// measure counting characters rather than screen columns produces.
func rowIsGlued(text string, width int) bool {
	rendered := formatRow(row{cells: []cell{{text, width}}, tail: gluedTail}, 0)
	if strings.Contains(rendered, "\n") {
		return false
	}
	return tailStartColumn(rendered, gluedTail) != width
}

// TestRowIsGluedUnitCases covers rowIsGlued directly, independent of any
// catalog. A field whose display width reaches its column takes its own line,
// so it is not glued, and that holds for plain ASCII at the column and for
// five double-width runes under the column in characters but at it on screen.
// A field under the column is padded to it, so it is not glued either.
//
// The last case is the detector's own arming. No output formatRow produces is
// glued once the padding counts screen columns, so a test built only on
// formatRow could not tell a working detector from one that always answers no.
// The hand-built line is what a rune-counting padder emits for the third case,
// and tailStartColumn has to report the column it really lands in.
func TestRowIsGluedUnitCases(t *testing.T) {
	const width = 10

	atWidth := strings.Repeat("a", width)
	if rowIsGlued(atWidth, width) {
		t.Errorf("a field whose display width equals the column takes its own line and must not be glued, got glued for %q (display width %d)",
			atWidth, displayWidth(atWidth))
	}

	underWidth := strings.Repeat("a", width-1)
	if rowIsGlued(underWidth, width) {
		t.Errorf("a field one column under its column is padded to it and must not be glued, got glued for %q (display width %d)",
			underWidth, displayWidth(underWidth))
	}

	// Five double-width runes: a rune count of 5, under the width of 10, and
	// a display width of 10, which reaches it.
	doubleWidthUnder := strings.Repeat("中", 5)
	if rowIsGlued(doubleWidthUnder, width) {
		t.Errorf("a field of double-width runes whose display width reaches its column takes its own line and must not be glued, got glued for %q (rune count %d, display width %d)",
			doubleWidthUnder, len([]rune(doubleWidthUnder)), displayWidth(doubleWidthUnder))
	}

	glued := doubleWidthUnder + strings.Repeat(" ", width-len([]rune(doubleWidthUnder))) + gluedTail
	if got := tailStartColumn(glued, gluedTail); got == width {
		t.Errorf("the hand-built rune-counted row puts its tail at column %d, so the detector cannot tell a glued row from a laid-out one: %q", got, glued)
	}
}

// TestChecksColumnNeverGluesInAnyLanguage sweeps every command's precondition
// list against every shipped locale: for every command in commands, every
// check verb.Checks(name) returns, and every tag msg.Tags() reports, the
// check's catalog key is rendered through that locale and asserted not
// glued at the 52-rune width verbHelp pads it to. Commands and checks are
// read from the running library rather than named here, and locales are
// discovered through msg.Tags(), which reads internal/msg/locales/*.json, so
// a new command, a new check, or a new shipped catalog is covered with no
// change to this test.
func TestChecksColumnNeverGluesInAnyLanguage(t *testing.T) {
	const width = 52
	for _, c := range commands {
		for _, check := range verb.Checks(c.name) {
			for _, tag := range msg.Tags() {
				rendered := msg.For(tag).T(check.Key)
				if rowIsGlued(rendered, width) {
					t.Errorf("command %s check %s locale %s: glued at width %d (rune count %d, display width %d): %q",
						c.name, check.Key, tag, width, len([]rune(rendered)), displayWidth(rendered), rendered)
				}
			}
		}
	}
}

// TestKindAndSubstateTokensNeverGlueInAnyLanguage sweeps the three state-kind
// tokens and the three card-substate tokens against every shipped locale,
// asserting none glues at the 10-rune width renderStates and renderListing
// pad them to.
func TestKindAndSubstateTokensNeverGlueInAnyLanguage(t *testing.T) {
	const width = 10
	names := []string{
		contract.KindIntake, contract.KindWork, contract.KindDone,
		contract.SubstateReady, contract.SubstateActive, contract.SubstateBlocked,
	}
	for _, name := range names {
		key := "token." + name
		for _, tag := range msg.Tags() {
			rendered := msg.For(tag).T(key)
			if rowIsGlued(rendered, width) {
				t.Errorf("token %s locale %s: glued at width %d (rune count %d, display width %d): %q",
					name, tag, width, len([]rune(rendered)), displayWidth(rendered), rendered)
			}
		}
	}
}

// TestEventTokensNeverGlueInAnyLanguage sweeps every journal event constant
// against every shipped locale, asserting none glues at the 14-rune width
// renderHistory pads the event column to.
func TestEventTokensNeverGlueInAnyLanguage(t *testing.T) {
	const width = 14
	events := []string{
		contract.EventCreated,
		contract.EventClaimed,
		contract.EventMoved,
		contract.EventReleased,
		contract.EventBlocked,
		contract.EventUnblocked,
		contract.EventExpired,
		contract.EventCommented,
		contract.EventAttached,
		contract.EventAttachmentReplaced,
		contract.EventAttachmentRemoved,
		contract.EventArchived,
		contract.EventRestored,
		contract.EventDeleted,
		contract.EventManualCorrection,
	}
	for _, event := range events {
		key := "token." + event
		for _, tag := range msg.Tags() {
			rendered := msg.For(tag).T(key)
			if rowIsGlued(rendered, width) {
				t.Errorf("event %s locale %s: glued at width %d (rune count %d, display width %d): %q",
					event, tag, width, len([]rune(rendered)), displayWidth(rendered), rendered)
			}
		}
	}
}
