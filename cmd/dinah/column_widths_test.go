package main

import (
	"os"
	"path/filepath"
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

// startColumnOf reports the display column a marker begins at in a line, or
// minus one when the line does not carry it.
func startColumnOf(line, marker string) int {
	at := strings.Index(line, marker)
	if at < 0 {
		return -1
	}
	return displayWidth(line[:at])
}

// TestHindiCommandHelpStartsEveryRefusalNameAtOneColumn asserts the case this
// card was filed over. dinah help add --lang hi draws three rows, each an
// ordinal three columns wide and a check sentence fifty-two wide, with the
// refusal name after them. Devanagari writes its vowels as combining marks and
// half of them take no column of their own, so a padder counting characters
// pays for every mark and comes up short: the three names began at display
// columns 52, 52 and 53 before this card.
//
// The literal is asserted as well as the agreement, since 57 is what the
// declared layout promises and three names agreeing on the wrong column is
// what the old measure produced.
func TestHindiCommandHelpStartsEveryRefusalNameAtOneColumn(t *testing.T) {
	root := newBench(t)
	got := runCLI(t, root, "help", "add", "--lang", "hi")
	if got.code != 0 {
		t.Fatalf("help add --lang hi: %d %s", got.code, got.errw)
	}
	names := []string{contract.Malformed, contract.UnknownState, contract.AtCapacity}
	found := 0
	for _, line := range strings.Split(got.out, "\n") {
		for _, name := range names {
			at := startColumnOf(line, name)
			if at < 0 {
				continue
			}
			found++
			if at != 57 {
				t.Errorf("the refusal name %s begins at display column %d and the declared layout puts it at 57:\n%q", name, at, line)
			}
		}
	}
	if found != len(names) {
		t.Errorf("found %d of the %d refusal rows, so this test asserts less than it claims", found, len(names))
	}
}

// TestEnglishCommandListStartsEverySummaryAtOneColumn asserts that every
// summary of the block bare dinah prints begins at display column 41, and that
// the two entries whose usage reaches the thirty-nine-column usage field
// continue on a line of their own instead of pushing their summary right.
func TestEnglishCommandListStartsEverySummaryAtOneColumn(t *testing.T) {
	got := runCLI(t, t.TempDir())
	if got.code != 0 {
		t.Fatalf("the help block: %d %s", got.code, got.errw)
	}
	lines := strings.Split(got.out, "\n")
	continued, summaries := 0, 0
	for _, c := range commands {
		if c.group == "" {
			continue
		}
		usage := verb.Usage(c.name)
		summary := msg.For(msg.Base).T("cmd." + c.name + ".summary")
		for i, line := range lines {
			if !strings.HasPrefix(line, "  "+usage) {
				continue
			}
			summaries++
			at := startColumnOf(line, summary)
			if at < 0 {
				continued++
				at = startColumnOf(lines[i+1], summary)
			}
			if at != 41 {
				t.Errorf("the summary of %s begins at display column %d and the declared layout puts it at 41", c.name, at)
			}
			break
		}
	}
	if summaries != 29 {
		t.Errorf("read %d command entries out of the block, want 29", summaries)
	}
	if continued != 2 {
		t.Errorf("%d entries continued on a line of their own, want the two whose usage reaches the column", continued)
	}
}

// TestAWideWorkbenchTitleStartsTheColumnsAfterItWhereTheyBelong asserts that a
// workbench titled in a script drawing two columns per rune starts the slug
// column at display column 34 and the path column at 50, which is where a row
// of Latin titles has always put them. A five-character title of this kind
// draws ten columns and counts as five characters, so a padder counting
// characters started the slug column at 39.
func TestAWideWorkbenchTitleStartsTheColumnsAfterItWhereTheyBelong(t *testing.T) {
	base := t.TempDir()
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	t.Setenv("DINAH_ACTOR", "alka")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_WORKBENCH", "")
	t.Setenv("COLUMNS", "")
	const title = "作業台管理"
	if displayWidth(title) != 10 {
		t.Fatalf("the fixture title draws %d columns, and this test is written for five wide characters", displayWidth(title))
	}
	root := filepath.Join(base, title)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := runCLI(t, root, "init", "--slug", "wb", "--operator", "alka"); got.code != 0 {
		t.Fatalf("init: %d %s", got.code, got.errw)
	}
	got := runCLI(t, root, "workbenches")
	if got.code != 0 {
		t.Fatalf("workbenches: %d %s", got.code, got.errw)
	}
	rows := 0
	for _, line := range strings.Split(got.out, "\n") {
		if !strings.Contains(line, title) {
			continue
		}
		rows++
		if at := startColumnOf(line, "wb"); at != 34 {
			t.Errorf("the slug column begins at display column %d and the declared layout puts it at 34:\n%q", at, line)
		}
		fields := strings.Fields(line)
		path := fields[len(fields)-1]
		if at := displayWidth(line) - displayWidth(path); at != 50 {
			t.Errorf("the path column begins at display column %d and the declared layout puts it at 50:\n%q", at, line)
		}
	}
	if rows != 1 {
		t.Errorf("the listing drew %d rows carrying the fixture title, want 1", rows)
	}
}

// TestAnUnusableWindowRendersUnbounded asserts what each shape of COLUMNS does
// to real output rather than to windowWidth alone. A value that states nothing
// a layout can use renders byte for byte as an unbounded window does, and a
// value too narrow to lay out renders as the narrowest one that can.
func TestAnUnusableWindowRendersUnbounded(t *testing.T) {
	root := newBench(t)
	render := func(columns string) string {
		t.Helper()
		t.Setenv("COLUMNS", columns)
		got := runCLI(t, root, "help", "move")
		if got.code != 0 {
			t.Fatalf("COLUMNS=%q: %d %s", columns, got.code, got.errw)
		}
		return got.out
	}
	unbounded := render("")
	for _, columns := range []string{"   ", "abc", "0", "-5"} {
		if got := render(columns); got != unbounded {
			t.Errorf("COLUMNS=%q renders differently from an unbounded window:\n%s", columns, diffLines(unbounded, got))
		}
	}
	narrowest := render("20")
	for _, columns := range []string{"3", "19"} {
		if got := render(columns); got != narrowest {
			t.Errorf("COLUMNS=%q renders differently from the narrowest window a layout can use:\n%s", columns, diffLines(narrowest, got))
		}
	}
}

// TestANarrowWindowClampsEveryContinuationLine asserts both bounds of the
// clamp on real output: at COLUMNS=40 no continuation line is indented past
// display column 20, and none is indented below its own row's indent.
func TestANarrowWindowClampsEveryContinuationLine(t *testing.T) {
	root := newBench(t)
	t.Setenv("COLUMNS", "40")
	got := runCLI(t, root, "help", "move")
	if got.code != 0 {
		t.Fatalf("help move: %d %s", got.code, got.errw)
	}
	continuations := 0
	for _, line := range strings.Split(got.out, "\n") {
		if strings.TrimSpace(line) == "" || !strings.HasPrefix(line, "   ") {
			continue
		}
		continuations++
		indent := displayWidth(line) - displayWidth(strings.TrimLeft(line, " "))
		if indent > 20 {
			t.Errorf("a continuation line is indented to display column %d, past the 20 the clamp allows:\n%q", indent, line)
		}
		if indent < 2 {
			t.Errorf("a continuation line is indented to display column %d, below its row's own indent:\n%q", indent, line)
		}
	}
	if continuations == 0 {
		t.Error("no continuation line was drawn, so this test asserts nothing about the clamp")
	}
}
