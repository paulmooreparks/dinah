package main

import (
	"os"
	"path/filepath"
	"strconv"
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
		contract.EventWorkbenchUpdated,
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
// ordinal, a check sentence and the refusal name after them. Devanagari writes
// its vowels as combining marks and half of them take no column of their own,
// so a padder counting characters pays for every mark and comes up short: the
// three names began at display columns 52, 52 and 53 before dinah-101.
//
// The column the names start in is computed here from the same catalog entries
// the head reads, since dinah-115 measures the block rather than declaring it:
// the indent, then the widest of the ordinals and their own heading, then the
// gutter, then the widest of the three check sentences and their heading, then
// the gutter again. A number typed into this test would assert what somebody
// once measured rather than what the rule produces.
func TestHindiCommandHelpStartsEveryRefusalNameAtOneColumn(t *testing.T) {
	root := newBench(t)
	got := runCLI(t, root, "help", "add", "--lang", "hi")
	if got.code != 0 {
		t.Fatalf("help add --lang hi: %d %s", got.code, got.errw)
	}
	hindi := msg.For("hi")
	order := displayWidth(hindi.T("column.help.order"))
	check := displayWidth(hindi.T("column.help.check"))
	checks := verb.Checks("add")
	if len(checks) != 3 {
		t.Fatalf("add declares %d checks, and this test is written for the three the profile carries", len(checks))
	}
	for i, one := range checks {
		if drawn := displayWidth(strconv.Itoa(i + 1)); drawn > order {
			order = drawn
		}
		if drawn := displayWidth(hindi.T(one.Key)); drawn > check {
			check = drawn
		}
	}
	want := 2 + order + 2 + check + 2
	names := []string{contract.Malformed, contract.UnknownState, contract.AtCapacity}
	found := 0
	for _, line := range strings.Split(got.out, "\n") {
		for _, name := range names {
			at := startColumnOf(line, name)
			if at < 0 {
				continue
			}
			found++
			if at != want {
				t.Errorf("the refusal name %s begins at display column %d and the measured layout puts it at %d:\n%q", name, at, want, line)
			}
		}
	}
	if found != len(names) {
		t.Errorf("found %d of the %d refusal rows, so this test asserts less than it claims", found, len(names))
	}
}

// TestEnglishCommandListStartsEverySummaryAtOneColumn asserts that every
// summary of the block bare dinah prints begins at the same display column,
// whether its own syntax fits on one line or wraps across several, since the
// syntax column now measures at the declared ceiling rather than at the
// width its own values need (dinah-200). The summary itself may also wrap,
// through the same wrapTail the arguments table already uses, and this
// checks the summary's own first line rather than the whole text for that
// reason.
//
// The column is computed from the rule the renderer follows rather than
// typed in: the indent, half of the window the block draws at (assumedWindow,
// since bare dinah draws with no width stated), and the gutter. A command
// whose usage is wider than that half needs more than one line for it, and
// this asserts the count of those against the six the fixture is known to
// carry, so a change to the command list that stops exercising the wrap is
// caught here rather than by a coincidence elsewhere. It separately counts
// how many summaries wrap, against no fixed number, since which summaries
// are long enough depends on the catalog text this test does not own; the
// count only has to be positive, which is what proves the interaction this
// test exists for is actually exercised.
func TestEnglishCommandListStartsEverySummaryAtOneColumn(t *testing.T) {
	got := runCLI(t, t.TempDir())
	if got.code != 0 {
		t.Fatalf("the help block: %d %s", got.code, got.errw)
	}
	room := halfWindow(assumedWindow)
	wrapIndent := 2 + ceilingContinuationIndent
	want := 2 + room + tableGutter
	lines := strings.Split(got.out, "\n")
	wrapped, summariesWrapped, summaries := 0, 0, 0
	for _, c := range commands {
		if c.group == "" {
			continue
		}
		usage := verb.Usage(c.name)
		summary := msg.For(msg.Base).T("cmd." + c.name + ".summary")
		first := strings.Split(firstChunk(usage, wrapIndent, room), "\n")[0]
		if first != usage {
			wrapped++
		}
		summaryFirst := strings.Split(breakTail(summary, want, assumedWindow), "\n")[0]
		if summaryFirst != summary {
			summariesWrapped++
		}
		found := false
		for _, line := range lines {
			if !strings.HasPrefix(line, "  "+first) {
				continue
			}
			found = true
			summaries++
			if at := startColumnOf(line, summaryFirst); at != want {
				t.Errorf("the summary of %s begins at display column %d and the ceiling puts it at %d", c.name, at, want)
			}
			break
		}
		if !found {
			t.Errorf("the block does not carry a first line for %s's syntax", c.name)
		}
	}
	if summaries != 36 {
		t.Errorf("read %d command entries out of the block, want 36", summaries)
	}
	if wrapped != 6 {
		t.Errorf("%d entries wrapped across more than one line, want the six whose syntax is wider than half the window", wrapped)
	}
	if summariesWrapped == 0 {
		t.Error("no summary wrapped across more than one line, so the tail-wrapping half of this shape is not exercised here")
	}
	if want != 44 {
		t.Errorf("the ceiling-bearing summary column measures %d, and this shape has started it at 44 since the ceiling was declared", want)
	}
}

// firstChunk is the rendering rule for a ceiling-bearing, wrapOptions-on table
// cell: a value that fits the room is written whole; a value that exceeds the
// room is broken on option boundaries when it carries them, otherwise on word
// boundaries. The test reads its own expected first chunk through this rule so
// it agrees with what the renderer draws, rather than recomputing the wrap
// shape by hand.
func firstChunk(value string, indent, room int) string {
	if displayWidth(value) <= room {
		return value
	}
	if hasOptionBoundary(value) {
		return breakOnOptions(value, indent, room)
	}
	return breakWords(value, indent, room)
}

// hasOptionBoundary is true when value carries at least one boundary the
// breakOnOptions rule recognises. The check is structural and exists to match
// splitOnOptionBoundaries's own view, so the two never disagree on what the
// rule applies to.
func hasOptionBoundary(value string) bool {
	depth := 0
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '[':
			if i+2 < len(value) && value[i+1] == '-' && value[i+2] == '-' {
				depth++
			}
		case ']':
			if depth > 0 {
				depth--
			}
		case ' ':
			if depth > 0 {
				continue
			}
			j := i + 1
			switch {
			case j+2 < len(value) && value[j] == '[' && value[j+1] == '-' && value[j+2] == '-':
				return true
			case j < len(value) && value[j] == '<':
				return true
			case j+1 < len(value) && value[j] == '-' && value[j+1] == '-':
				return true
			}
		}
	}
	return false
}

// TestAWideWorkbenchTitleStartsTheColumnsAfterItWhereTheyBelong asserts that a
// workbench titled in a script drawing two columns per rune starts the columns
// after it where the measure puts them. A five-character title of this kind
// draws ten columns and counts as five characters, so a padder counting
// characters started the slug column five columns late.
//
// Both columns are computed from the fixture's own values and the headings the
// catalog serves, since dinah-115 measures this block: the title draws ten and
// its heading nine, so the slug starts at 14, and the slug draws two and its
// heading four, so the path starts at 20.
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
	english := msg.For(msg.Base)
	workbench := displayWidth(title)
	if heading := displayWidth(english.T("column.workbenches.workbench")); heading > workbench {
		workbench = heading
	}
	slug := displayWidth("wb")
	if heading := displayWidth(english.T("column.workbenches.slug")); heading > slug {
		slug = heading
	}
	wantSlug := 2 + workbench + 2
	wantPath := wantSlug + slug + 2
	rows := 0
	for _, line := range strings.Split(got.out, "\n") {
		if !strings.Contains(line, title) {
			continue
		}
		rows++
		if at := startColumnOf(line, "wb"); at != wantSlug {
			t.Errorf("the slug column begins at display column %d and the measured layout puts it at %d:\n%q", at, wantSlug, line)
		}
		fields := strings.Fields(line)
		path := fields[len(fields)-1]
		if at := displayWidth(line) - displayWidth(path); at != wantPath {
			t.Errorf("the path column begins at display column %d and the measured layout puts it at %d:\n%q", at, wantPath, line)
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
// clamp at a window of 40: no continuation line is indented past display
// column 20, and none is indented below its own row's indent.
//
// The block it renders holds a field wider than the window itself, which is
// the case the clamp still governs. A block whose columns stand at their
// headings while a field under one of them reaches its column no longer draws
// a continuation line at all, since it stacks.
func TestANarrowWindowClampsEveryContinuationLine(t *testing.T) {
	drawn := tableSession(40).tableLines(table{
		indent:  2,
		columns: headed("Command", "What it does"),
		rows: rowsOf(
			[]string{"add <title> [--state]", "file a card"},
			[]string{strings.Repeat("x", 60), "does a thing"},
		),
	})
	continuations := 0
	for _, line := range drawn {
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
