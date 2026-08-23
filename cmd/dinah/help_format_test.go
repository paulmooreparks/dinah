package main

import "testing"

// The two blocks below are the operator's own desired output from
// docs/specs/dinah-220-help-formatting-ux-sketch.md, brought forward to the
// command set the tool actually ships. Every row the sketch draws is here
// byte for byte; the rows that differ from it are the ones the trunk added
// after he drew it (the `card` command, `add`'s severity and priority flags,
// and the `--help` and `--version` rows of the global-flag table), plus the
// capitalized flag summaries he asked for when he answered the card's first
// open question.
//
// This sits alongside testdata/help.txt rather than replacing it: that
// fixture covers the assumed window, and these two cover the widths the
// card's own defects show up at. A narrow window exercises the cap, the
// option packing and the interleaved wrap all at once; a wide one exercises
// the measured column position, which no other test in the tree pins.

const desiredHelpAt107 = `Dinah keeps work moving.

Usage: dinah <command> [arguments]

WORK
  add <title> [--state <state>] [--severity <level>]     File a new card in the first state
    [--priority <level>]
  claim <card> [--expires <duration>]                    Take up a ready card
  move <card> <state> [--override]                       Carry a card to another state
  pull [state] [--no-claim] [--expires <duration>]       Claim the head of a state's queue and move it
    [--override]                                           there in one act
  release <card>                                         Give the card back to its queue
  block <card> <reason> [--kind <kind>]                  Raise an obstacle and free the card
  unblock <card>                                         Lift a block (operator only)
  comment <card> <text|->                                Record a comment on a card
  attach <ref> <file> [--description <text>]             Attach a file, or replace its bytes
    [--replace]
  join <card> <workstream>                               Add a card to a workstream
  leave <card> <workstream>                              Take a card out of a workstream
  archive <ref>                                          Move a card, a state, or anything below a card,
                                                           out of the live set
  delete <ref> --yes                                     Destroy a card, a state, or anything below a card,
                                                           along with its history
  rename <ref> <name>                                    Rename an attachment
  card <get|set> <card> <field> [value]                  Read one of a card's own fields, or write one

READ
  status                                                 Where this workbench stands, and what you hold
  states                                                 The flow, in order
  ls [state] [--ready]                                   The cards of a state, in queue order
  next [state]                                           The card a state offers next
  query [query]                                          The cards of the workbench that match a query
  tree [query] [--group-by <axes>] [--depth <level>]     The workbench's cards nested along a chain of axes
  contents <ref> [--depth <level>]                       What an entity of the workbench contains
  show <ref>                                             A card, or anything below it
  log <card>                                             The recorded actions of a card, oldest first
  instructions <card|state>                              The instructions served at a position
  guide [topic]                                          The embedded guides, or one of them

WORKBENCH
  init [dir] [--from <source>] [--slug <slug>]           Create a workbench here, optionally from a
    [--operator <actor>]                                   template
  export                                                 Write this workbench's interchange form to stdout
  extract <dir>                                          Copy this workbench's definition out as a template
  path <ref>                                             Print the file path of this workbench, of a card,
                                                           or of anything below a card
  edit <ref>                                             Open this workbench, a card, or anything below a
                                                           card in your editor
  config [get|set] [key] [value]                         List your user settings, or read or write one
  check [--finish] [--migrate-ordinals]                  Look for structural defects in this workbench
    [--migrate-slugs] [--migrate-states]
    [--migrate-workstreams]
  whoami                                                 The actor your actions carry, and whether it is
                                                           the operator
  workbench [get|set] [field] [value] [--yes]            Read this workbench's own fields, or write one
  workstream [new|get|set] [workstream|title] [field]    Read this workbench's workstreams, create one, or
    [value] [--yes]                                        write one's fields
  workbenches                                            The workbenches reachable from here
  version [--catalogs]                                   What Dinah is and what it conforms to

SERVE
  mcp [--root <dir>]                                     Serve workbenches over MCP on stdio

Global flags:
  Option             What it does
  -----------------  --------------------------------------------------------------------
  --workbench <dir>  Use this workbench instead of the one discovered from here
  --json             Emit the canonical machine form
  --quiet            Suppress served instructions on claim and move
  --lang <tag>       Render in this language; run ` + "`" + `dinah version --catalogs` + "`" + ` for the tags
  --actor <name>     Act as this owner
  --help, -h, -?     Print this help, or one command's page when a command is named
  --version, -V      Print the version and stop

Environment: DINAH_WORKBENCH, DINAH_HOME, DINAH_FORMAT=json, DINAH_LANG, DINAH_ACTOR, DINAH_EDITOR, DINAH_MCP_ROOT, VISUAL, EDITOR

Exit codes: 0 ok, 2 refused, 3 stale, 4 unreachable.

Run 'dinah help <command>' for one command's arguments, what can go wrong, and its exit codes.
Run 'dinah guide' for the guides Dinah carries, and read the quick start at https://github.com/paulmooreparks/dinah/blob/main/docs/quick-start.md
`

const desiredHelpAt200 = `Dinah keeps work moving.

Usage: dinah <command> [arguments]

WORK
  add <title> [--state <state>] [--severity <level>] [--priority <level>]                             File a new card in the first state
  claim <card> [--expires <duration>]                                                                 Take up a ready card
  move <card> <state> [--override]                                                                    Carry a card to another state
  pull [state] [--no-claim] [--expires <duration>] [--override]                                       Claim the head of a state's queue and move it there in one act
  release <card>                                                                                      Give the card back to its queue
  block <card> <reason> [--kind <kind>]                                                               Raise an obstacle and free the card
  unblock <card>                                                                                      Lift a block (operator only)
  comment <card> <text|->                                                                             Record a comment on a card
  attach <ref> <file> [--description <text>] [--replace]                                              Attach a file, or replace its bytes
  join <card> <workstream>                                                                            Add a card to a workstream
  leave <card> <workstream>                                                                           Take a card out of a workstream
  archive <ref>                                                                                       Move a card, a state, or anything below a card, out of the live set
  delete <ref> --yes                                                                                  Destroy a card, a state, or anything below a card, along with its history
  rename <ref> <name>                                                                                 Rename an attachment
  card <get|set> <card> <field> [value]                                                               Read one of a card's own fields, or write one

READ
  status                                                                                              Where this workbench stands, and what you hold
  states                                                                                              The flow, in order
  ls [state] [--ready]                                                                                The cards of a state, in queue order
  next [state]                                                                                        The card a state offers next
  query [query]                                                                                       The cards of the workbench that match a query
  tree [query] [--group-by <axes>] [--depth <level>]                                                  The workbench's cards nested along a chain of axes
  contents <ref> [--depth <level>]                                                                    What an entity of the workbench contains
  show <ref>                                                                                          A card, or anything below it
  log <card>                                                                                          The recorded actions of a card, oldest first
  instructions <card|state>                                                                           The instructions served at a position
  guide [topic]                                                                                       The embedded guides, or one of them

WORKBENCH
  init [dir] [--from <source>] [--slug <slug>] [--operator <actor>]                                   Create a workbench here, optionally from a template
  export                                                                                              Write this workbench's interchange form to stdout
  extract <dir>                                                                                       Copy this workbench's definition out as a template
  path <ref>                                                                                          Print the file path of this workbench, of a card, or of anything below a card
  edit <ref>                                                                                          Open this workbench, a card, or anything below a card in your editor
  config [get|set] [key] [value]                                                                      List your user settings, or read or write one
  check [--finish] [--migrate-ordinals] [--migrate-slugs] [--migrate-states] [--migrate-workstreams]  Look for structural defects in this workbench
  whoami                                                                                              The actor your actions carry, and whether it is the operator
  workbench [get|set] [field] [value] [--yes]                                                         Read this workbench's own fields, or write one
  workstream [new|get|set] [workstream|title] [field] [value] [--yes]                                 Read this workbench's workstreams, create one, or write one's fields
  workbenches                                                                                         The workbenches reachable from here
  version [--catalogs]                                                                                What Dinah is and what it conforms to

SERVE
  mcp [--root <dir>]                                                                                  Serve workbenches over MCP on stdio

Global flags:
  Option             What it does
  -----------------  --------------------------------------------------------------------
  --workbench <dir>  Use this workbench instead of the one discovered from here
  --json             Emit the canonical machine form
  --quiet            Suppress served instructions on claim and move
  --lang <tag>       Render in this language; run ` + "`" + `dinah version --catalogs` + "`" + ` for the tags
  --actor <name>     Act as this owner
  --help, -h, -?     Print this help, or one command's page when a command is named
  --version, -V      Print the version and stop

Environment: DINAH_WORKBENCH, DINAH_HOME, DINAH_FORMAT=json, DINAH_LANG, DINAH_ACTOR, DINAH_EDITOR, DINAH_MCP_ROOT, VISUAL, EDITOR

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
	for _, window := range []int{40, 50, 60, 70, 80, 90, 100, 107, 120, 160, 200} {
		s := tableSession(window)
		for _, line := range helpListingLines(t, s) {
			if drawn := displayWidth(line); drawn > window {
				t.Errorf("at a window of %d a listing line draws %d columns wide:\n%s", window, drawn, line)
			}
		}
	}
}
