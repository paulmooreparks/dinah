# Dinah Help Formatting

This document calls out a number of problems with help-display formatting in Dinah. Note that the actual help content may have changed by the time an agent is assigned to correct these issues. Do not modify the content of the help! Constrain your fixes to the formatting as described herein.

## Current format

This is the help display on a terminal that is 107 characters wide. This is wider than most, and yet the formatting is quite bad. I'll call out specific examples further below.

```
Dinah keeps work moving.

Usage: dinah <command> [arguments]

WORK
  add <title> [--state <state>]                          file a new card in the first state
  claim <card> [--expires <duration>]                    take up a ready card
  move <card> <state> [--override]                       carry a card to another state
  release <card>                                         give the card back to its queue
  block <card> <reason> [--kind <kind>]                  raise an obstacle and free the card
  unblock <card>                                         lift a block (operator only)
  comment <card> <text|->                                record a comment on a card
  attach                                                 attach a file, or replace its bytes
    <ref>
    <file>
    [--description <text>]
    [--replace]
  join <card> <workstream>                               add a card to a workstream
  leave <card> <workstream>                              take a card out of a workstream
  archive <ref>                                          move a card, a state, or anything below a card,
                                                         out of the live set
  delete <ref> --yes                                     destroy a card, a state, or anything below a
                                                         card, along with its history
  rename <ref> <name>                                    rename an attachment

READ
  status                                                 where this workbench stands, and what you hold
  states                                                 the flow, in order
  ls [state] [--ready]                                   the cards of a state, in queue order
  next [state]                                           the card a state offers next
  query [query]                                          the cards of the workbench that match a query
  tree [query] [--group-by <axes>] [--depth <level>]     the workbench's cards nested along a chain of
                                                         axes
  contents <ref> [--depth <level>]                       what an entity of the workbench contains
  show <ref>                                             a card, or anything below it
  log <card>                                             the recorded actions of a card, oldest first
  instructions <card|state>                              the instructions served at a position
  guide [topic]                                          the embedded guides, or one of them

WORKBENCH
  init [dir]                                             create a workbench here, optionally from a
    [--from <source>]
    [--slug <slug>]
    [--operator <actor>]
                                                         template
  export                                                 write this workbench's interchange form to stdout
  extract <dir>                                          copy this workbench's definition out as a
                                                         template
  path <ref>                                             print the file path of this workbench, of a card,
                                                         or of anything below a card
  edit <ref>                                             open this workbench, a card, or anything below a
                                                         card in your editor
  config [get|set] [key] [value]                         list your user settings, or read or write one
  check                                                  look for structural defects in this workbench
    [--finish]
    [--migrate-ordinals]
    [--migrate-slugs]
    [--migrate-states]
    [--migrate-workstreams]
  whoami                                                 the actor your actions carry, and whether it is
                                                         the operator
  workbench [get|set] [field] [value] [--yes]            read this workbench's own fields, or write one
  workstream [new|get|set] [workstream|title] [field]    read this workbench's workstreams, create one, or
    [value]
    [--yes]
                                                         write one's fields
  workbenches                                            the workbenches reachable from here
  version [--catalogs]                                   what Dinah is and what it conforms to

SERVE
  mcp [--root <dir>]                                     serve workbenches over MCP on stdio

Global flags:
  Option             What it does
  -----------------  --------------------------------------------------------------------
  --workbench <dir>  use this workbench instead of the one discovered from here
  --json             emit the canonical machine form
  --quiet            suppress served instructions on claim and move
  --lang <tag>       render in this language; run `dinah version --catalogs` for the tags
  --actor <name>     act as this owner

Environment: DINAH_WORKBENCH, DINAH_HOME, DINAH_FORMAT=json, DINAH_LANG, DINAH_ACTOR, DINAH_EDITOR, DINAH_MCP_ROOT, VISUAL, EDITOR

Exit codes: 0 ok, 2 refused, 3 stale, 4 unreachable.

Run 'dinah help <command>' for one command's arguments, what can go wrong, and its exit codes.
Run 'dinah guide' for the guides Dinah carries, and read the quick start at https://github.com/paulmooreparks/dinah/blob/main/docs/quick-start.md```
```

### Incorrect Capitalization of English Translation

Description lines are not capitalized in English. This should be fixed, and other translations (currently only Hindi and German) should follow their relevant rules accordingly.

Current (wrong):

```
  workbench [get|set] [field] [value] [--yes]            read this workbench's own fields, or write one
```

Desired:

```
  workbench [get|set] [field] [value] [--yes]            Read this workbench's own fields, or write one
```

### Option Wrapping

The options listed for commands do not need to be wrapped onto their own lines. They should be rendered so that lines break between options.

Current (wrong):

```
  attach                                                 attach a file, or replace its bytes
    <ref>
    <file>
    [--description <text>]
    [--replace]
```

Desired:

```
  attach <ref> <file> [--description <text>]             Attach a file, or replace its bytes
    [--replace]
```

### Command Description Wrapping

The description for a command does not wrap correctly. There can be a large gap. There is also no indentation on the wrapped description lines

Current (wrong):

```
  workstream [new|get|set] [workstream|title] [field]    read this workbench's workstreams, create one, or
    [value]
    [--yes]
                                                         write one's fields
```

Desired:

```
  workstream [new|get|set] [workstream|title] [field]    Read this workbench's workstreams, create one, or
    [value]  [--yes]                                       write one's fields
```

### Display Too Wide

When the terminal is very wide, the help display takes up too much space. The description column should be placed based on the width of the widest line of the command column.

Current (wrong):

```
Dinah keeps work moving.

Usage: dinah <command> [arguments]

WORK
  add <title> [--state <state>]                                                                                          file a new card in the first state
  claim <card> [--expires <duration>]                                                                                    take up a ready card
  move <card> <state> [--override]                                                                                       carry a card to another state
  release <card>                                                                                                         give the card back to its queue
  block <card> <reason> [--kind <kind>]                                                                                  raise an obstacle and free the card
  unblock <card>                                                                                                         lift a block (operator only)
  comment <card> <text|->                                                                                                record a comment on a card
  attach <ref> <file> [--description <text>] [--replace]                                                                 attach a file, or replace its bytes
  join <card> <workstream>                                                                                               add a card to a workstream
  leave <card> <workstream>                                                                                              take a card out of a workstream
  archive <ref>                                                                                                          move a card, a state, or anything below a card, out of the live set
  delete <ref> --yes                                                                                                     destroy a card, a state, or anything below a card, along with its history
  rename <ref> <name>                                                                                                    rename an attachment

READ
  status                                                                                                                 where this workbench stands, and what you hold
  states                                                                                                                 the flow, in order
  ls [state] [--ready]                                                                                                   the cards of a state, in queue order
  next [state]                                                                                                           the card a state offers next
  query [query]                                                                                                          the cards of the workbench that match a query
  tree [query] [--group-by <axes>] [--depth <level>]                                                                     the workbench's cards nested along a chain of axes
  contents <ref> [--depth <level>]                                                                                       what an entity of the workbench contains
  show <ref>                                                                                                             a card, or anything below it
  log <card>                                                                                                             the recorded actions of a card, oldest first
  instructions <card|state>                                                                                              the instructions served at a position
  guide [topic]                                                                                                          the embedded guides, or one of them

WORKBENCH
  init [dir] [--from <source>] [--slug <slug>] [--operator <actor>]                                                      create a workbench here, optionally from a template
  export                                                                                                                 write this workbench's interchange form to stdout
  extract <dir>                                                                                                          copy this workbench's definition out as a template
  path <ref>                                                                                                             print the file path of this workbench, of a card, or of anything below a card
  edit <ref>                                                                                                             open this workbench, a card, or anything below a card in your editor
  config [get|set] [key] [value]                                                                                         list your user settings, or read or write one
  check [--finish] [--migrate-ordinals] [--migrate-slugs] [--migrate-states] [--migrate-workstreams]                     look for structural defects in this workbench
  whoami                                                                                                                 the actor your actions carry, and whether it is the operator
  workbench [get|set] [field] [value] [--yes]                                                                            read this workbench's own fields, or write one
  workstream [new|get|set] [workstream|title] [field] [value] [--yes]                                                    read this workbench's workstreams, create one, or write one's fields
  workbenches                                                                                                            the workbenches reachable from here
  version [--catalogs]                                                                                                   what Dinah is and what it conforms to

SERVE
  mcp [--root <dir>]                                                                                                     serve workbenches over MCP on stdio

Global flags:
  Option             What it does
  -----------------  --------------------------------------------------------------------
  --workbench <dir>  use this workbench instead of the one discovered from here
  --json             emit the canonical machine form
  --quiet            suppress served instructions on claim and move
  --lang <tag>       render in this language; run `dinah version --catalogs` for the tags
  --actor <name>     act as this owner

Environment: DINAH_WORKBENCH, DINAH_HOME, DINAH_FORMAT=json, DINAH_LANG, DINAH_ACTOR, DINAH_EDITOR, DINAH_MCP_ROOT, VISUAL, EDITOR

Exit codes: 0 ok, 2 refused, 3 stale, 4 unreachable.

Run 'dinah help <command>' for one command's arguments, what can go wrong, and its exit codes.
Run 'dinah guide' for the guides Dinah carries, and read the quick start at https://github.com/paulmooreparks/dinah/blob/main/docs/quick-start.md
```

Desired:

```
Dinah keeps work moving.

Usage: dinah <command> [arguments]

WORK
  add <title> [--state <state>]                                                                       File a new card in the first state
  claim <card> [--expires <duration>]                                                                 Take up a ready card
  move <card> <state> [--override]                                                                    Carry a card to another state
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
  --workbench <dir>  use this workbench instead of the one discovered from here
  --json             emit the canonical machine form
  --quiet            suppress served instructions on claim and move
  --lang <tag>       render in this language; run `dinah version --catalogs` for the tags
  --actor <name>     act as this owner

Environment: DINAH_WORKBENCH, DINAH_HOME, DINAH_FORMAT=json, DINAH_LANG, DINAH_ACTOR, DINAH_EDITOR, DINAH_MCP_ROOT, VISUAL, EDITOR

Exit codes: 0 ok, 2 refused, 3 stale, 4 unreachable.

Run 'dinah help <command>' for one command's arguments, what can go wrong, and its exit codes.
Run 'dinah guide' for the guides Dinah carries, and read the quick start at https://github.com/paulmooreparks/dinah/blob/main/docs/quick-start.md
```
