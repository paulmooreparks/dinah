# UX sketch: what the help pages say after this card

Every block below is drawn at an eighty-column window, which is what Dinah measures against when nothing states a width, so a piped run and an eighty-column terminal both print exactly these bytes. Blocks headed **Today** came out of a binary built at commit `7719580` and run against a scratch workbench. Blocks headed **After** are what this card asks for, laid out through the renderer rather than by hand.

Machine vocabulary is unchanged throughout. Command names, flag names, refusal names, and the words inside angle brackets all stay as they are.

Two rules govern every drawn table.

The left cell spells the argument exactly as the syntax line above it spells it, brackets and all, because one function composes both. An optional positional keeps its square brackets, so `ls` reads `[state]` and `move` reads `<state>`, and you can see which one is required without reading a word. The left column is headed `As you write it` rather than `Argument`, because a cell reading `<ref>` is a spelling rather than a name.

The right cell wraps at the window and its continuation lines indent under the column. Dinah does not wrap a table's last column today; it writes the cell whole and lets your terminal fold it at column zero. This card teaches the renderer to break the last column on word boundaries, as an opt-in the arguments table asks for and no other table does, so every table Dinah prints today prints identically after this card.

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
  As you write it       What it is
  --------------------  --------------------------------------------------------
  [--finish]            complete or roll back a structural act that was
                        interrupted
  [--migrate-ordinals]  stamp a creation ordinal on every entity that carries
                        none
  [--migrate-slugs]     derive a slug for every state of this workbench that
                        carries none
  [--migrate-states]    remove stranded identifiers from this workbench's own
                        list of states

Exit codes: 0 ok, 2 refused, 3 stale, 4 unreachable.
```

## attach

`attach` accepts more than the pages, and the previous drawing of this sketch, said it does. It resolves through `ResolveEntity` and carries no refusal after it, so this workbench and a state are both legal subjects, and so is the empty reference, which Dinah reads as this workbench.

Today:

```
attach <ref> <file> [--replace]

attach a file, or replace its bytes
```

After:

```
attach <ref> <file> [--description <text>] [--replace]

attach a file, or replace its bytes

What you may write:
  As you write it         What it is
  ----------------------  ------------------------------------------------------
  <ref>                   what the file hangs off: this workbench, a state, a
                          card, or a comment or an attachment below a card; with
                          --replace, the attachment whose bytes you are
                          replacing
  <file>                  a path on this machine to the file you are attaching
  [--description <text>]  a line describing the attachment, stored beside it
  [--replace]             replace an existing attachment's bytes instead of
                          adding one

For more, run `dinah guide references`.
```

## ls

The vocabulary is your own workbench's states, so Dinah prints it only when a workbench resolves from where you stand:

```
ls [state] [--ready]

the cards of a state, in queue order

What you may write:
  As you write it  What it is
  ---------------  -------------------------------------------------------------
  [state]          which state to list, also written --state <state>; every
                   state when you name none (one of: intake, doing, done)
  [--ready]        list only the cards that are ready to be claimed
```

If no workbench resolves from where you stand, Dinah drops the parenthesis and prints the rest of the row, so `dinah help ls` answers anywhere.

## init

```
init [dir] [--from <source>] [--slug <slug>] [--operator <actor>]

create a workbench here, optionally from a template

What you may write:
  As you write it       What it is
  --------------------  --------------------------------------------------------
  [dir]                 where to create the workbench; the directory you are in
                        when you name none
  [--from <source>]     a directory holding a workbench, or a single file
                        written by `dinah export` or `dinah extract`
  [--slug <slug>]       the prefix every card reference carries; derived from
                        the directory's name when you name none
  [--operator <actor>]  who owns this workbench; your own actor when you name
                        none
```

## claim

```
What you may write:
  As you write it         What it is
  ----------------------  ------------------------------------------------------
  <card>                  the card you are taking up
  [--expires <duration>]  how long your claim holds before it goes stale,
                          written as a number and a unit: 30m, 2h, 7d
```

`claim` takes a card rather than a reference, so its page carries no pointer to the references guide.

## block

```
What you may write:
  As you write it  What it is
  ---------------  -------------------------------------------------------------
  <card>           the card you are blocking
  <reason>         why it is blocked, in one shell word or in quotation marks
  [--kind <kind>]  your own word for what sort of obstacle this is; Dinah stores
                   whatever you write and checks it against no set
```

## query

```
What you may write:
  As you write it  What it is
  ---------------  -------------------------------------------------------------
  [query]          the query, in one shell word or in quotation marks; every
                   live card when you write none

For more, run `dinah guide query`.
```

## show, path, edit, and instructions

Four commands take a reference, under four spellings, and today no page says that the four vocabularies differ. Each of these pages ends with a pointer to `dinah guide references`.

```
show <card|path>

What you may write:
  As you write it  What it is
  ---------------  -------------------------------------------------------------
  <card|path>      a card, or something below a card; show does not take this
                   workbench
```

```
path <card|path>

What you may write:
  As you write it  What it is
  ---------------  -------------------------------------------------
  <card|path>      this workbench, a card, or something below a card
```

`edit` draws the same row as `path`, because the two share one resolver.

```
instructions <card|state>

What you may write:
  As you write it  What it is
  ---------------  -------------------------------------------------------------
  <card|state>     a card or a state; instructions takes neither this workbench
                   nor anything below a card
```

## guide and config

Two arguments carry a set that is fixed rather than read from your workbench:

```
What you may write:
  As you write it  What it is
  ---------------  -------------------------------------------------------------
  [topic]          which guide to read; the topics when you name none (one of:
                   getting-started, query, references, verbs, workbench-layout)
```

```
What you may write:
  As you write it  What it is
  ---------------  -------------------------------------------------------------
  [get|set]        read one setting or write one; every setting with its value
                   when you name none
  [key]            which setting (one of: lang, actor, editor, workbench)
  [value]          what to store under it, on a set
```

The `[get|set]` cell carries no appended set, because its own spelling already names both values.

## The top-level block, four lines

Everything else stays, including the command order and the four group headings.

```
  init [dir] [--from <source>] [--slug <slug>] [--operator <actor>]
  attach <ref> <file> [--description <text>] [--replace]
  --lang <tag>   render in this language; run `dinah version --catalogs` for the tags

Environment: DINAH_WORKBENCH, DINAH_HOME, DINAH_FORMAT=json, DINAH_LANG,
             DINAH_ACTOR, DINAH_EDITOR, VISUAL, EDITOR
```

Dinah walks `DINAH_EDITOR`, then `VISUAL`, then `EDITOR` to find your editor, so the line as it stands today is false rather than merely short.

## dinah guide references

A new guide carries the grammar that today lives only in the resolvers. Its first screen:

```
You name a thing to Dinah by writing a reference. A reference names this
workbench, a state, a card, or something that hangs off a card, and you write
it as a path with slashes between its parts.

This workbench, two spellings that mean the same thing:

    dinah path workbench
    dinah path .

A card, by its reference:

    dinah show wb-1

A state, by its slug, its name, or its identifier:

    dinah attach doing notes.md

Something below a card:

    dinah path wb-1/card          the card's own file, which wb-1 alone gives you
    dinah path wb-1/journal       everything that has happened to the card
    dinah path wb-1/comments      every comment on the card
    dinah path wb-1/comments/1    one comment
    dinah path wb-1/checklist     every checklist item
    dinah path wb-1/checklist/1   one checklist item
    dinah path wb-1/attachments   every attachment
    dinah path wb-1/attachments/1 one attachment

You may write three shorter spellings that select one kind of checklist item:

    dinah path wb-1/oq            the open questions
    dinah path wb-1/ac            the acceptance criteria
    dinah path wb-1/d             the decisions

You may write an entity's own identifier in place of its number. The number
counts in the order the entities were created, which is not always the order a
listing prints them in.

If the collection you name is empty, Dinah tells you that nothing answers to
the reference rather than telling you the collection is empty.
```

That last paragraph is one self-contained line of prose and no more, so the card that fixes the refusal deletes a line rather than editing a page.

And the table that settles which command takes what:

```
| Command      | Workbench | State | Card | Below a card |
|--------------|-----------|-------|------|--------------|
| path         | yes       | no    | yes  | yes          |
| edit         | yes       | no    | yes  | yes          |
| show         | no        | no    | yes  | yes          |
| instructions | no        | yes   | yes  | no           |
| attach       | yes       | yes   | yes  | yes          |
| archive      | no        | yes   | yes  | yes          |
| delete       | no        | yes   | yes  | yes          |
```

Every row of that table was provoked against the build at `7719580` rather than read off a resolver, because the previous drawing of it was read off a resolver and was wrong.
