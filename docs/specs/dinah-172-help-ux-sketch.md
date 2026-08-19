# dinah-172: what the help pages say after this card

Every transcript below is the intended output. The "today" blocks were produced by
building commit 7719580 and running the binary against a scratch workbench; the
"after" blocks are what this card asks for. Machine vocabulary (command names, flag
names, refusal names, and the words inside angle brackets) is unchanged.

## The new section on a per-command page

Each page grows one section, headed `What you may write:`, between the summary and
the refusal table. It is a two-column table drawn by the renderer the rest of the
help already uses. The left cell is the argument as the syntax line spells it. The
right cell is the argument's summary, with its vocabulary appended in parentheses
when the argument has a closed one.

## check

Today:

```
check [--finish] [--migrate-ordinals] [--migrate-slugs] [--migrate-states]

look for structural defects in this workbench

Exit codes: 0 ok, 2 refused, 3 stale, 4 unreachable.
```

After:

```
check [--finish] [--migrate-ordinals] [--migrate-slugs] [--migrate-states]

look for structural defects in this workbench

Dinah exits 2 when it finds a defect, so a script may read the exit code alone.

What you may write:
  Argument            What it is
  ------------------  -------------------------------------------------------------
  --finish            complete or roll back a structural act that was interrupted
  --migrate-ordinals  stamp a creation ordinal on every entity that carries none
  --migrate-slugs     derive a slug for every state of the workbench that carries none
  --migrate-states    remove stranded identifiers from the workbench's list of states

Exit codes: 0 ok, 2 refused, 3 stale, 4 unreachable.
```

The sentence under the summary comes from `cmd.check.note`, an optional per-command
key any command may carry. Only `check` carries one after this card.

## attach

Today the page shows two of the three flags the command accepts, and the reference
it asks for is the card even when you are replacing an attachment:

```
attach <ref> <file> [--replace]

attach a file, or replace its bytes
```

After:

```
attach <ref> <file> [--description <text>] [--replace]

attach a file, or replace its bytes

What you may write:
  Argument              What it is
  --------------------  -----------------------------------------------------------
  ref                   the card the file hangs off, or, with --replace, the
                        attachment whose bytes you are replacing
  file                  a path on this machine to the file you are attaching
  --description <text>  a line describing the attachment, stored beside it
  --replace             replace an existing attachment's bytes instead of adding one

What can go wrong, in the order each is checked:
  Order  What can go wrong                        Refusal
  -----  ---------------------------------------  ------------------
  1      the reference and the file both resolve  dinah.unknown-path
  2      the request names an owner               no-owner

For more, run `dinah guide references`.

Exit codes: 0 ok, 2 refused, 3 stale, 4 unreachable.
```

## show, path, and edit

The three take a reference, print the same display name, and accept three different
vocabularies. Today nothing on any page says so. After this card each page says
what its own command takes:

```
show <card|path>

a card, or anything below it

What you may write:
  Argument   What it is
  ---------  --------------------------------------------------------------
  card|path  a card, or something below a card; not the workbench itself
```

```
path <card|path>

print the file path of this workbench, of a card, or of anything below a card

What you may write:
  Argument   What it is
  ---------  --------------------------------------------------------------
  card|path  this workbench, a card, or something below a card
```

```
edit <card|path>

open this workbench, a card, or anything below a card in your editor

What you may write:
  Argument   What it is
  ---------  --------------------------------------------------------------
  card|path  this workbench, a card, or something below a card
```

Every one of the three ends with a pointer line reading: For more, run
`dinah guide references`.

## ls

`ls` takes a state as a positional word and also accepts it as `--state`, which no
page says today. The vocabulary is the workbench's own states, so Dinah prints it
only when a workbench resolves from where you are:

```
ls [state] [--ready]

the cards of a state, in queue order

What you may write:
  Argument  What it is
  --------  ----------------------------------------------------------------
  state     which state to list, also written --state <state>; every state
            when you name none (one of: intake, doing, done)
  --ready   list only the cards that are ready to be claimed
```

Run the same command outside a workbench and the parenthesis is absent, because
Dinah has no states to name:

```
  state     which state to list, also written --state <state>; every state
            when you name none
```

## init

Today `init` takes a positional directory the syntax line does not carry, and
`--from` names one shape while accepting two. After:

```
init [dir] [--from <source>] [--slug <slug>] [--operator <actor>]

create a workbench here, optionally from a template

What you may write:
  Argument            What it is
  ------------------  ------------------------------------------------------------
  dir                 where to create the workbench; the current directory when you
                      name none
  --from <source>     a directory holding a workbench, or a single file written by
                      `dinah export` or `dinah extract`
  --slug <slug>       the prefix every card reference carries; derived from the
                      directory's name when you name none
  --operator <actor>  who owns this workbench; your own actor when you name none
```

## claim

`--expires` takes Go's duration syntax with a day suffix Go itself does not have,
so `7d` is accepted and `1w` is refused. Today the format appears in one example in
a guide no page points at:

```
claim <card> [--expires <duration>]

take up a ready card

What you may write:
  Argument             What it is
  -------------------  --------------------------------------------------------
  card                 the card you are taking up
  --expires <duration>  how long your claim holds before it goes stale, as a
                        number and a unit: 30m, 2h, 7d

For more, run `dinah guide references`.
```

## block

`--kind` reads like the checked vocabulary `<state>` on the line above it and is
free text:

```
block <card> <reason> [--kind <kind>]

raise an obstacle and free the card

What you may write:
  Argument       What it is
  -------------  ---------------------------------------------------------------
  card           the card you are blocking
  reason         why, in your own words, in one shell word or in quotation marks
  --kind <kind>  your own word for what sort of obstacle this is; Dinah stores
                 whatever you write and checks it against nothing
```

## query

The page names three vocabulary checks and no field, operator, or value. The guide
that carries all of them gains a pointer:

```
query [query]

the cards of the workbench that match a query

What you may write:
  Argument  What it is
  --------  ------------------------------------------------------------------
  query     the query, in one shell word or in quotation marks; every live card
            when you write none

What can go wrong, in the order each is checked:
  ... unchanged ...

For more, run `dinah guide query`.
```

## The top-level block

Four lines of the block change. Everything else, including the order of the
commands and the four group headings, stays as it is.

```
  init [dir] [--from <source>] [--slug <slug>] [--operator <actor>]
                                         create a workbench here, optionally from a template
```

```
  attach <ref> <file> [--description <text>] [--replace]
                                         attach a file, or replace its bytes
```

```
  --lang <tag>       render in this language; run `dinah version --catalogs` for the tags
```

```
Environment: DINAH_WORKBENCH, DINAH_HOME, DINAH_FORMAT=json, DINAH_LANG, DINAH_ACTOR, DINAH_EDITOR, VISUAL, EDITOR
```

`VISUAL` and `EDITOR` are two rungs of the ladder `internal/bench/config.go` walks
to find your editor, so a reader whose editor opens the wrong program can learn
from the help which variable is involved.

## dinah guide references

A new guide carries the grammar the audit read out of the two resolvers. Its first
screen:

```
# References: how you name a thing to Dinah

Several of Dinah's commands take a reference rather than a card. A reference names
the workbench, a state, a card, or something that hangs off a card, and you write
it as a path with slashes between its parts.

## The workbench

You may write the workbench two ways, and both mean the same thing:

    dinah path workbench
    dinah path .

## A card

You name a card by its reference, which is the workbench's slug, a hyphen, and the
card's number:

    dinah show wb-1

## Something below a card

Add a slash and the part you want:

    dinah path wb-1/card          the card's own file, which is what wb-1 alone gives you
    dinah path wb-1/journal       everything that has happened to the card
    dinah path wb-1/comments/1    one comment
    dinah path wb-1/checklist/1   one checklist item
    dinah path wb-1/attachments/1 one attachment

Three shorter spellings select one kind of checklist item rather than the whole
list:

    dinah path wb-1/oq            the open questions
    dinah path wb-1/ac            the acceptance criteria
    dinah path wb-1/d             the decisions

You may write an entity's own identifier in place of its number. The number counts
in the order the entities were created, which is not always the order a listing
prints them in.

## Which commands take which

Dinah has two reference vocabularies, and the table says which command reads which.

| Command | Workbench | State | Card | Below a card |
|---|---|---|---|---|
| `path` | yes | no | yes | yes |
| `edit` | yes | no | yes | yes |
| `show` | no | no | yes | yes |
| `attach` | no | no | yes | yes |
| `archive` | no | yes | yes | yes |
| `delete` | no | yes | yes | yes |
```

The guide is one embedded Markdown file like the four that already ship, so it is
served in English under every language setting, as the existing guides are.
