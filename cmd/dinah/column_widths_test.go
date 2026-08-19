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
// summary of the block bare dinah prints begins at one display column, and
// that the two entries whose syntax cannot be laid out inside the window
// continue on a line of their own instead of pushing their summary right.
//
// The column is computed from the fixture's own syntax lines rather than typed
// in: it is the indent, the widest syntax among the entries that fit an
// eighty-column window packed tight, and the gutter. That comes to 41, which
// is where the declared width of 39 started every summary before dinah-115,
// and computing it is what makes this test read the rule rather than the
// number a previous measurement produced.
func TestEnglishCommandListStartsEverySummaryAtOneColumn(t *testing.T) {
	got := runCLI(t, t.TempDir())
	if got.code != 0 {
		t.Fatalf("the help block: %d %s", got.code, got.errw)
	}
	want := 2 + widestFittingSyntax(t) + 2
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
			if at != want {
				t.Errorf("the summary of %s begins at display column %d and the measured layout puts it at %d", c.name, at, want)
			}
			break
		}
	}
	if summaries != 33 {
		t.Errorf("read %d command entries out of the block, want 33", summaries)
	}
	if continued != 4 {
		t.Errorf("%d entries continued on a line of their own, want the four whose syntax cannot be laid out inside the window", continued)
	}
	if want != 41 {
		t.Errorf("the measured summary column is %d, and every block of this shape has started it at 41 since the width was declared", want)
	}
}

// widestFittingSyntax is the widest syntax line among the command entries that
// can be laid out inside an eighty-column window with their fields packed
// tight. An entry that cannot be laid out inside the window however the
// columns are chosen does not get to widen the column, so the four long
// syntax lines are left out of this the same way the table leaves them out.
func widestFittingSyntax(t *testing.T) int {
	t.Helper()
	widest := displayWidth(msg.For(msg.Base).T("column.commands.command"))
	fits := 0
	for _, c := range commands {
		if c.group == "" {
			continue
		}
		usage := verb.Usage(c.name)
		summary := msg.For(msg.Base).T("cmd." + c.name + ".summary")
		if 2+displayWidth(usage)+2+displayWidth(summary) > 80 {
			continue
		}
		fits++
		if drawn := displayWidth(usage); drawn > widest {
			widest = drawn
		}
	}
	if fits == 0 {
		t.Fatal("no command entry fits the window, so this measure asserts nothing")
	}
	return widest
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
