package main

import (
	"strings"
	"testing"

	"dinah/internal/msg"
	"dinah/internal/verb"
)

// The two blocks below are the operator's own desired output from
// docs/specs/dinah-220-help-formatting-ux-sketch.md, brought forward to the
// command set the tool actually ships. Every row the sketch draws is here
// byte for byte; the rows that differ from it are the ones the trunk added
// after he drew it (the `card` and `changes` commands, `add`'s severity and
// priority flags, and the `--help` and `--version` rows of the global-flag
// table), plus the capitalized flag summaries he asked for when he answered
// the card's first open question.
//
// This sits alongside testdata/help.txt rather than replacing it: that
// fixture covers the assumed window, and these two cover the widths the
// card's own defects show up at. A narrow window exercises the cap, the
// option packing and the interleaved wrap all at once; a wide one exercises
// the measured column position, which no other test in the tree pins.

const desiredHelpAt107 = `Dinah keeps work moving.

Usage: dinah <command> [arguments]

WORK
  add <title> [--column <column>] [--severity <level>]   File a new card in the first column
    [--priority <level>]
  claim <card> [--expires <duration>]                    Take up a ready card
  move <card> <column> [--override]                      Carry a card to another column
  pull [column] [--no-claim] [--expires <duration>]      Claim the head of a column's queue and move it
    [--override]                                           there in one act
  release <card>                                         Give the card back to its queue
  block <card> <reason> [--kind <kind>]                  Raise an obstacle and free the card
  unblock <card>                                         Lift a block (operator only)
  comment <card> <text|->                                Record a comment on a card
  attach <ref> <file> [--description <text>]             Attach a file, or replace its bytes
    [--replace]
  join <card> <workstream>                               Add a card to a workstream
  leave <card> <workstream>                              Take a card out of a workstream
  archive <ref>                                          Move a card, a column, or anything below a card,
                                                           out of the live set
  delete <ref> --yes                                     Destroy a card, a column, or anything below a
                                                           card, along with its history
  rename <ref> <name>                                    Rename an attachment
  card <get|set> <card> <field> [value]                  Read one of a card's own fields, or write one

READ
  status [--root <path>] [--max-depth <n>]               Where this workbench stands, and what you hold
  columns                                                The flow, in order
  ls [column] [--ready] [--root <path>]                  The cards of a column, in queue order
    [--max-depth <n>]
  next [column] [--root <path>] [--max-depth <n>]        The card a column offers next
  query [query]                                          The cards of the workbench that match a query
  search <phrase> [--query <terms>] [--archived]         Every place a phrase occurs in this workbench
    [--root <path>] [--max-depth <n>]
  tree [query] [--group-by <axes>] [--depth <level>]     The workbench's cards nested along a chain of axes
    [--root <path>] [--max-depth <n>]
  contents <ref> [--depth <level>]                       What an entity of the workbench contains
  attachments [ref]                                      What is attached to an entity of the workbench
  show <ref>                                             A card, or anything below it
  log <card>                                             The recorded actions of a card, oldest first
  changes [--since <cursor>] [--card <ref>]              What has happened on this workbench since a cursor
    [--column <column>] [--root <path>] [--max-depth <n>]
  instructions <card|column>                             The instructions served at a position
  guide [topic]                                          The embedded guides, or one of them

WORKBENCH
  init [dir] [--from <source>] [--slug <slug>]           Create a workbench here, optionally from a
    [--operator <actor>]                                   template
  export                                                 Write this workbench's interchange form to stdout
  extract <dir>                                          Copy this workbench's definition out as a template
  reshape --from <source> [--map <retired=destination>]  Carry this workbench to the column layout a new
    [--yes]                                                definition declares
  path <ref>                                             Print the file path of this workbench, of a card,
                                                           or of anything below a card
  edit <ref>                                             Open this workbench, a card, or anything below a
                                                           card in your editor
  config [get|set] [key] [value]                         List your user settings, or read or write one
  check [--finish] [--migrate-ordinals]                  Look for structural defects in this workbench
    [--migrate-slugs] [--migrate-columns]
    [--migrate-vocabulary] [--migrate-container]
    [--remint <dir>] [--migrate-workstreams] [--witness]
    [--yes] [--root <path>] [--max-depth <n>]
  whoami                                                 The actor your actions carry, and whether it is
                                                           the operator
  workbench [get|set] [field] [value] [--yes]            Read this workbench's own fields, or write one
  workstream [new|get|set] [workstream|title] [field]    Read this workbench's workstreams, create one, or
    [value] [--slug <slug>] [--yes]                        write one's fields
  column <new> <title> [--kind <kind>] [--capacity <n>]  Create a column in this workbench's flow
    [--slug <slug>] [--before <column>]
  workbenches [path] [--max-depth <n>]                   The workbenches beneath a directory, or the ones
                                                           reachable from here
  version [--catalogs]                                   What Dinah is and what it conforms to

SERVE
  mcp [--root <dir>]                                     Serve workbenches over MCP on stdio

Global flags:
  Option             What it does
  -----------------  --------------------------------------------------------------------
  --workbench <dir>  Use this workbench instead of the one discovered from here
  --json             Emit the canonical machine form
  --format <name>    Select json or compact for the machine form
  --quiet            Suppress served instructions on claim and move
  --lang <tag>       Render in this language; run ` + "`" + `dinah version --catalogs` + "`" + ` for the tags
  --actor <name>     Act as this owner
  --help, -h, -?     Print this help, or one command's page when a command is named
  --version, -V      Print the version and stop

Environment: DINAH_WORKBENCH, DINAH_HOME, DINAH_FORMAT=json|compact, DINAH_LANG, DINAH_ACTOR, DINAH_EDITOR, DINAH_MCP_ROOT, VISUAL, EDITOR

Exit codes: 0 ok, 2 refused, 3 stale, 4 unreachable.

Run 'dinah help <command>' for one command's arguments, what can go wrong, and its exit codes.
Run 'dinah guide' for the guides Dinah carries, and read the quick start at https://github.com/paulmooreparks/dinah/blob/main/docs/quick-start.md
`

const desiredHelpAt200 = `Dinah keeps work moving.

Usage: dinah <command> [arguments]

WORK
  add <title> [--column <column>] [--severity <level>] [--priority <level>]                        File a new card in the first column
  claim <card> [--expires <duration>]                                                              Take up a ready card
  move <card> <column> [--override]                                                                Carry a card to another column
  pull [column] [--no-claim] [--expires <duration>] [--override]                                   Claim the head of a column's queue and move it there in one act
  release <card>                                                                                   Give the card back to its queue
  block <card> <reason> [--kind <kind>]                                                            Raise an obstacle and free the card
  unblock <card>                                                                                   Lift a block (operator only)
  comment <card> <text|->                                                                          Record a comment on a card
  attach <ref> <file> [--description <text>] [--replace]                                           Attach a file, or replace its bytes
  join <card> <workstream>                                                                         Add a card to a workstream
  leave <card> <workstream>                                                                        Take a card out of a workstream
  archive <ref>                                                                                    Move a card, a column, or anything below a card, out of the live set
  delete <ref> --yes                                                                               Destroy a card, a column, or anything below a card, along with its history
  rename <ref> <name>                                                                              Rename an attachment
  card <get|set> <card> <field> [value]                                                            Read one of a card's own fields, or write one

READ
  status [--root <path>] [--max-depth <n>]                                                         Where this workbench stands, and what you hold
  columns                                                                                          The flow, in order
  ls [column] [--ready] [--root <path>] [--max-depth <n>]                                          The cards of a column, in queue order
  next [column] [--root <path>] [--max-depth <n>]                                                  The card a column offers next
  query [query]                                                                                    The cards of the workbench that match a query
  search <phrase> [--query <terms>] [--archived] [--root <path>] [--max-depth <n>]                 Every place a phrase occurs in this workbench
  tree [query] [--group-by <axes>] [--depth <level>] [--root <path>] [--max-depth <n>]             The workbench's cards nested along a chain of axes
  contents <ref> [--depth <level>]                                                                 What an entity of the workbench contains
  attachments [ref]                                                                                What is attached to an entity of the workbench
  show <ref>                                                                                       A card, or anything below it
  log <card>                                                                                       The recorded actions of a card, oldest first
  changes [--since <cursor>] [--card <ref>] [--column <column>] [--root <path>] [--max-depth <n>]  What has happened on this workbench since a cursor
  instructions <card|column>                                                                       The instructions served at a position
  guide [topic]                                                                                    The embedded guides, or one of them

WORKBENCH
  init [dir] [--from <source>] [--slug <slug>] [--operator <actor>]                                Create a workbench here, optionally from a template
  export                                                                                           Write this workbench's interchange form to stdout
  extract <dir>                                                                                    Copy this workbench's definition out as a template
  reshape --from <source> [--map <retired=destination>] [--yes]                                    Carry this workbench to the column layout a new definition declares
  path <ref>                                                                                       Print the file path of this workbench, of a card, or of anything below a card
  edit <ref>                                                                                       Open this workbench, a card, or anything below a card in your editor
  config [get|set] [key] [value]                                                                   List your user settings, or read or write one
  check [--finish] [--migrate-ordinals] [--migrate-slugs] [--migrate-columns]                      Look for structural defects in this workbench
    [--migrate-vocabulary] [--migrate-container] [--remint <dir>] [--migrate-workstreams]
    [--witness] [--yes] [--root <path>] [--max-depth <n>]
  whoami                                                                                           The actor your actions carry, and whether it is the operator
  workbench [get|set] [field] [value] [--yes]                                                      Read this workbench's own fields, or write one
  workstream [new|get|set] [workstream|title] [field] [value] [--slug <slug>] [--yes]              Read this workbench's workstreams, create one, or write one's fields
  column <new> <title> [--kind <kind>] [--capacity <n>] [--slug <slug>] [--before <column>]        Create a column in this workbench's flow
  workbenches [path] [--max-depth <n>]                                                             The workbenches beneath a directory, or the ones reachable from here
  version [--catalogs]                                                                             What Dinah is and what it conforms to

SERVE
  mcp [--root <dir>]                                                                               Serve workbenches over MCP on stdio

Global flags:
  Option             What it does
  -----------------  --------------------------------------------------------------------
  --workbench <dir>  Use this workbench instead of the one discovered from here
  --json             Emit the canonical machine form
  --format <name>    Select json or compact for the machine form
  --quiet            Suppress served instructions on claim and move
  --lang <tag>       Render in this language; run ` + "`" + `dinah version --catalogs` + "`" + ` for the tags
  --actor <name>     Act as this owner
  --help, -h, -?     Print this help, or one command's page when a command is named
  --version, -V      Print the version and stop

Environment: DINAH_WORKBENCH, DINAH_HOME, DINAH_FORMAT=json|compact, DINAH_LANG, DINAH_ACTOR, DINAH_EDITOR, DINAH_MCP_ROOT, VISUAL, EDITOR

Exit codes: 0 ok, 2 refused, 3 stale, 4 unreachable.

Run 'dinah help <command>' for one command's arguments, what can go wrong, and its exit codes.
Run 'dinah guide' for the guides Dinah carries, and read the quick start at https://github.com/paulmooreparks/dinah/blob/main/docs/quick-start.md
`

// TestHelpAtANarrowWindowMatchesTheSketch asserts the whole block at the
// width the operator captured his sketch at: capitalized summaries, the
// `attach` row packed onto one line with `[--replace]` continuing under it,
// and the `workstream` row drawing its syntax continuation and its
// description continuation on one physical line.
//
// What the test guards against is a later change to the wrap that fixes one
// row and breaks another, which reading a single row's assertion would let
// through. The comparison is byte for byte, so trailing whitespace on a
// continuation line fails it too.
func TestHelpAtANarrowWindowMatchesTheSketch(t *testing.T) {
	if got := tableSession(107).helpBlock(); got != desiredHelpAt107 {
		t.Errorf("the help block at a window of 107 differs from the desired block:\n%s", diffLines(desiredHelpAt107, got))
	}
}

// TestHelpAtAWideWindowMatchesTheSketch asserts the other end of the range:
// at a window of 200 the description column starts two columns past the
// widest command line rather than at half the window, so the wide terminal
// the operator complained about no longer strands every description across a
// river of blank space.
func TestHelpAtAWideWindowMatchesTheSketch(t *testing.T) {
	if got := tableSession(200).helpBlock(); got != desiredHelpAt200 {
		t.Errorf("the help block at a window of 200 differs from the desired block:\n%s", diffLines(desiredHelpAt200, got))
	}
}

// helpListingLines cuts the command listing out of the whole help block, by
// the two catalog lines that bracket it: the usage line above it and the
// global-flags heading below it. Reading the boundary out of the catalog
// keeps the cut honest when a heading is reworded, and a boundary that goes
// missing fails the test rather than silently returning a shorter block.
func helpListingLines(t *testing.T, s *session) []string {
	t.Helper()
	lines := splitLines(s.helpBlock())
	first, last := -1, -1
	for i, line := range lines {
		switch line {
		case s.r.T("help.usage"):
			first = i + 1
		case s.r.T("help.flags"):
			last = i
		}
	}
	if first < 0 || last < 0 || last < first {
		t.Fatalf("the help block does not carry a command listing between %q and %q", s.r.T("help.usage"), s.r.T("help.flags"))
	}
	return lines[first:last]
}

// helpSession is a session that renders one language at a stated window.
// The width sweeps below use it to draw the same page in every catalog the
// binary carries, because a width assertion taken in the base language alone
// says nothing about a catalog whose words are longer.
func helpSession(window int, tag string) *session {
	return &session{r: msg.For(tag), width: window}
}

// helpSweepWindows are the windows the two sweeps below draw at: every width from
// 24 up to 120, then two wide ones.
//
// The floor is 24 rather than a rounder number because that is the narrowest
// window this tool lays anything out at, and because a floor above the widths
// where a defect lives is the same failure as testing two widths and calling
// the range covered. This card's first blocker showed at 50 and 80 and not at
// 107 or 200; its second showed on the per-command pages at 34, 35, 36, 37,
// 41 and 42, all of them under the floor of 40 the first sweep was written
// with. Every width in between is drawn rather than sampled, since it is the
// widths nobody thought to pick that these defects have twice hidden at.
func helpSweepWindows() []int {
	windows := make([]int, 0, 100)
	for window := 24; window <= 120; window++ {
		windows = append(windows, window)
	}
	return append(windows, 160, 200)
}

// reachesPastTheWindow reports whether a drawn line runs past the window on
// text the renderer could have broken.
//
// A line the renderer could not break is over the edge because one unit of
// it is wider than the room it had, which is the renderer's own documented
// case: breakWords writes a word wider than its room whole rather than
// splitting it, so that a reference inside a value stays copyable, and
// packTokens only ever starts a line with such a unit. What these sweeps are
// for is the other case, a line the packer would have broken had it been
// measuring the room the line is actually drawn into.
//
// Which unit is unbreakable depends on the axis the fragment came from, so
// the fragment is asked rather than assumed, through the renderer's own
// splitter rather than through a second copy of its rule. A fragment
// carrying an option boundary is a packed run of option groups and could
// have broken at one. A fragment that is a single option group is atomic
// however wide it is. A fragment carrying no option group at all is prose,
// and prose breaks between words.
func reachesPastTheWindow(fragment string, begins, window int) bool {
	if fragment == "" {
		return false
	}
	pieces := splitOnOptionBoundaries(fragment)
	unit := pieces[0]
	if len(pieces) == 1 && !isOptionShaped(unit) {
		unit = strings.Fields(fragment)[0]
	}
	return begins+displayWidth(unit) <= window
}

// edgeFragment is the run of text that reaches a drawn listing line's right
// edge, and the display column it begins at. It reports false when the line
// carries nothing a wrap could have helped.
//
// A listing line can carry two fields, the capped column's own text and the
// summary beside it, and only the second of them can reach the right edge.
// Which columns each occupies is not guessed back out of the ink: begins is
// read off the laid-out table, and a continuation line's summary draws
// ceilingContinuationIndent further right, exactly as ceilingRowLine draws
// it.
//
// A line whose capped column has run into the summary's own column is the
// one case this steps around. Only a piece wider than the cap can push it
// there, since every other piece is wrapped to the cap, and such a piece is
// written whole by the renderer's own rule. The line is over the edge
// because the piece is, which is not what a width sweep is looking for.
func edgeFragment(line string, indent, begins, window int) (string, int, bool) {
	if displayWidth(line) <= window {
		return "", 0, false
	}
	summaryAt := begins
	if drawnIndent := displayWidth(line) - displayWidth(strings.TrimLeft(line, " ")); drawnIndent > indent {
		summaryAt = begins + ceilingContinuationIndent
	}
	if displayWidth(line) <= summaryAt {
		trimmed := strings.TrimLeft(line, " ")
		return trimmed, displayWidth(line) - displayWidth(trimmed), true
	}
	before, after := splitAtColumn(line, summaryAt)
	if !strings.HasSuffix(before, " ") {
		return "", 0, false
	}
	fragment := strings.TrimLeft(after, " ")
	return fragment, summaryAt + displayWidth(after) - displayWidth(fragment), true
}

// splitAtColumn cuts a drawn line at a display column, returning what draws
// before it and what draws from it. A column past the end of the line
// returns the whole line and nothing after it.
func splitAtColumn(line string, column int) (string, string) {
	drawn := 0
	for at, r := range line {
		if drawn >= column {
			return line[:at], line[at:]
		}
		drawn += displayWidth(string(r))
	}
	return line, ""
}

// TestNoListingLineReachesPastTheWindow sweeps the command listing across a
// spread of windows and refuses any line that draws past the edge.
//
// It exists because the card that moved the description column's
// continuation two columns to the right measured that continuation for the
// room it used to have, so a continuation that filled its line ran two
// columns past the window. Every criterion on that card, both golden blocks
// and the regenerated testdata/help.txt all passed anyway: the goldens sit
// at 107 and 200, and those are two of the widths where the words happen to
// stop short of the edge. A defect a fixture records rather than catches is
// what this sweep is for, so the widths below deliberately include narrow
// ones, where the wrap runs most often and the overrun showed on thirteen
// lines at once.
//
// The listing alone is measured, not the whole block. The global-flags
// table declares no wrap and its rows run past a narrow window on the trunk
// exactly as they do here (four rows and four footer sentences at a window
// of fifty, all of them older than this test), so sweeping the whole block
// would assert something this renderer has never done. The listing is the
// one table that asks to wrap, which makes it the one table that can be
// held to the window.
func TestNoListingLineReachesPastTheWindow(t *testing.T) {
	for _, tag := range msg.Tags() {
		for _, window := range helpSweepWindows() {
			s := helpSession(window, tag)
			laid := s.layOut(withGuides(s.commandListing()))
			begins := laid.indent + laid.widths[laid.ceilingColumn] + tableGutter
			for _, line := range helpListingLines(t, s) {
				fragment, at, measurable := edgeFragment(line, laid.indent, begins, window)
				if measurable && reachesPastTheWindow(fragment, at, window) {
					t.Errorf("in %s at a window of %d a listing line draws %d columns wide:\n%s", tag, window, displayWidth(line), line)
				}
			}
		}
	}
}

// TestTheCommandListingNeverStacks holds the listing to its table at every
// window the sweep draws, including the narrow ones.
//
// A stacked block draws each field whole, on a line of its own under its
// column's heading, and nothing in that form wraps. So a listing that stacks
// at a narrow window escapes the window by drawing lines wider than any
// window: at 33 columns it drew 87. The capped column is exempt from the
// stack rule because ceilingRowLine wraps its value rather than letting one
// value take the rest of the line, and this is that exemption held in place.
// Before it, this card had moved the threshold from below 32 to below 34,
// which is how the two-column band was found at all.
func TestTheCommandListingNeverStacks(t *testing.T) {
	for _, tag := range msg.Tags() {
		for _, window := range helpSweepWindows() {
			s := helpSession(window, tag)
			if laid := s.layOut(withGuides(s.commandListing())); laid.stacks() {
				t.Errorf("in %s at a window of %d the command listing gives up its table for a stack", tag, window)
			}
		}
	}
}

// commandSyntaxLines is the syntax line one command's help page draws at
// this session's window, and its continuations, split into lines.
//
// It draws the line the way verbHelp draws it, through renderSyntaxLine at
// the page's own indent, rather than by rendering the whole page: every
// section under the syntax line reads the workbench, and a width sweep has
// no workbench to read. The indent is the constant verbHelp itself passes,
// so the two cannot drift apart.
//
// The syntax line alone is measured, for the reason the listing alone is:
// the sections below it draw tables that declare no wrap and sentences the
// renderer never breaks, and they run past a narrow window on the trunk
// exactly as they do here. The syntax line is the one part of the page that
// asks to wrap, which makes it the one part that can be held to the window.
func commandSyntaxLines(s *session, name string) []string {
	return splitLines(s.renderSyntaxLine(verb.Usage(name), syntaxContinuationIndent))
}

// TestNoSyntaxLineReachesPastTheWindow sweeps every command's own help page
// across the same windows and refuses a syntax line drawn past the edge.
//
// It exists because the fix for the listing's own overrun was applied where
// the defect was reported and not to the class it belongs to.
// renderSyntaxLine is the sibling call site: it wrapped the syntax to the
// whole window and then drew every continuation two columns further right,
// so a continuation that filled its line ran two columns past the edge, in
// exactly the shape the listing's had. It went unnoticed because before this
// card the syntax line drew one option group per line and a continuation
// never filled its room; the greedy packing is what fills it. Measured
// against the binary, the branch drew past the window at twelve
// command-and-width pairs that the trunk drew cleanly, none of them wider
// than 42 columns.
func TestNoSyntaxLineReachesPastTheWindow(t *testing.T) {
	for _, tag := range msg.Tags() {
		for _, window := range helpSweepWindows() {
			s := helpSession(window, tag)
			for _, c := range commands {
				for _, line := range commandSyntaxLines(s, c.name) {
					if displayWidth(line) <= window {
						continue
					}
					trimmed := strings.TrimLeft(line, " ")
					at := displayWidth(line) - displayWidth(trimmed)
					if reachesPastTheWindow(trimmed, at, window) {
						t.Errorf("in %s at a window of %d the syntax line of %s draws %d columns wide:\n%s", tag, window, c.name, displayWidth(line), line)
					}
				}
			}
		}
	}
}
