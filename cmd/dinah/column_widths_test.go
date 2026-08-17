package main

import (
	"strings"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// displayWidth reports how many terminal columns a field occupies: two for
// every rune from a script East Asian typography renders at double width, one
// for every other rune. The range table is hand-rolled rather than pulled
// from a dependency (decision D-4 on dinah-85): go.mod carries none today,
// and this test only needs to recognise the blocks a translated catalog
// could plausibly carry.
func displayWidth(field string) int {
	width := 0
	for _, r := range field {
		if isDoubleWidth(r) {
			width += 2
			continue
		}
		width++
	}
	return width
}

// isDoubleWidth reports whether a rune belongs to CJK Unified Ideographs, CJK
// Symbols and Punctuation, Hiragana, Katakana, Hangul Syllables, or the
// Fullwidth Forms block, the scripts a translated catalog could plausibly
// carry that a plain rune count under-measures on screen.
func isDoubleWidth(r rune) bool {
	switch {
	case r >= 0x3000 && r <= 0x303F:
		return true
	case r >= 0x3040 && r <= 0x309F:
		return true
	case r >= 0x30A0 && r <= 0x30FF:
		return true
	case r >= 0x4E00 && r <= 0x9FFF:
		return true
	case r >= 0xAC00 && r <= 0xD7A3:
		return true
	case r >= 0xFF00 && r <= 0xFFEF:
		return true
	default:
		return false
	}
}

// rowIsGlued renders cell through the product's own alignedRow rather than
// recomputing what alignedRow ought to do, then inspects what came back. A
// cell alignedRow wraps onto its own continuation line is correctly handled
// by construction and is never glued, whatever its length. A cell it does
// not wrap is glued exactly when its true screen width still reaches the
// column, which is the one case alignedRow's rune-count trigger misses and
// pad()'s rune-based spacing cannot make up for.
func rowIsGlued(cell string, width int) bool {
	rendered := alignedRow("", []paddedCell{{cell, width}}, "TAIL")
	if strings.Contains(rendered, "\n") {
		return false
	}
	return displayWidth(cell) >= width
}

// TestDisplayWidth checks the three cases the spec names directly: plain
// ASCII counts runes, a field of double-width runes counts twice its rune
// count, and a field mixing both sums the two measures.
func TestDisplayWidth(t *testing.T) {
	cases := []struct {
		name  string
		field string
		want  int
	}{
		{"plain ascii", "hello", 5},
		{"double-width CJK", "中文字", 6},
		{"mixed ascii and double-width", "ab中文", 6},
	}
	for _, c := range cases {
		if got := displayWidth(c.field); got != c.want {
			t.Errorf("%s: displayWidth(%q) = %d, want %d", c.name, c.field, got, c.want)
		}
	}
}

// TestRowIsGluedUnitCases covers rowIsGlued directly, independent of any
// catalog: a field whose rune count exactly equals the width, which
// alignedRow wraps onto its own line and so must not be glued; a field one
// rune under the width in plain ASCII, which pad() gives a full column's
// worth of spacing and so must not be glued; and a field built from
// double-width runes whose rune count sits under the width but whose display
// width reaches it, which alignedRow does not wrap and so must be glued.
// This third case is the one this test exists to catch, and no shipped
// catalog reaches it today, so this unit case is the only proof the branch
// works at all.
func TestRowIsGluedUnitCases(t *testing.T) {
	const width = 10

	atWidth := strings.Repeat("a", width)
	if rowIsGlued(atWidth, width) {
		t.Errorf("a field whose rune count equals the width is wrapped by alignedRow onto its own line and must not be glued, got glued for %q (rune count %d, display width %d)",
			atWidth, len([]rune(atWidth)), displayWidth(atWidth))
	}

	underWidth := strings.Repeat("a", width-1)
	if rowIsGlued(underWidth, width) {
		t.Errorf("a plain ASCII field one rune under the width is padded by pad(), not glued, got glued for %q (rune count %d, display width %d)",
			underWidth, len([]rune(underWidth)), displayWidth(underWidth))
	}

	// Five double-width runes: a rune count of 5, under the width of 10, so
	// alignedRow's rune-count trigger does not wrap it, but a display width
	// of 10 still reaches the column.
	doubleWidthUnder := strings.Repeat("中", 5)
	if !rowIsGlued(doubleWidthUnder, width) {
		t.Errorf("a field of double-width runes whose rune count sits under the width but whose display width reaches it is not wrapped by alignedRow and must be glued, got not glued for %q (rune count %d, display width %d)",
			doubleWidthUnder, len([]rune(doubleWidthUnder)), displayWidth(doubleWidthUnder))
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
