# References

You name a thing to Dinah by writing a reference, and the language a
reference is written in is called DinahPath. DinahPath addresses things
and never filters them, and every reference is one address rooted at
something that names itself, so it takes no brackets, no wildcards, no
functions, and no axes. Those four are examples of what those two rules
leave out rather than the whole of it. When you want Dinah to find things
for you instead of naming one yourself, write a query, and `dinah guide
query` teaches how.

A reference names this workbench, a workstream, a column, a card, or
something that hangs off one of those, and you write it as a path with
slashes between its parts.

## This workbench

You may write this workbench in two ways, and the two mean the same thing:

    dinah path workbench
    dinah path .

Your workbench's slug reaches the same place, so `wb/attachments/1` and
`workbench/attachments/1` name one file.

## A card

You write a card as its reference, which is your workbench's slug and the
card's number:

    dinah show wb-1

## A column

You write a column as its slug, its name, or its identifier:

    dinah attach doing notes.md

## A workstream

You write a workstream as the word workstream, a slash, and the
workstream's slug or its identifier:

    dinah contents workstream/addressing

## Something below a card

You write something below a card as the card's reference, a slash, and the
name of what you want:

    dinah path wb-1/card             the card's own file, which wb-1 alone gives you
    dinah path wb-1/journal          everything that has happened to the card
    dinah path wb-1/comments         every comment on the card
    dinah path wb-1/comments/1       one comment
    dinah path wb-1/checklist        every checklist item
    dinah path wb-1/checklist/1      one checklist item
    dinah path wb-1/attachments      every attachment
    dinah path wb-1/attachments/1    one attachment
    dinah path wb-1/attachments/1/payload
                                     the file the attachment carries

Three more spellings each select one kind of checklist item:

    dinah path wb-1/questions        the open questions
    dinah path wb-1/criteria         the acceptance criteria
    dinah path wb-1/decisions        the decisions

Dinah also accepts `oq`, `ac`, and `d` for those same three, which is what
it printed and taught before the words landed. It never writes one of them
back, so a reference you read off a screen carries the word.

## Naming a whole collection

A reference that stops at a collection and picks nothing out of it names
every live member of that collection:

    dinah show wb-1/comments

A collection that holds nothing is an empty answer rather than a mistake.
Which commands take a reference of that shape is settled one command at a
time, in the table below, because an act that writes to a whole collection
cannot be undone and Dinah has no restore.

## The number and the identifier

You may write an entity's own identifier in place of its number, and you
may write an attachment's filename in place of its number. Dinah tries the
identifier first, then the position, then the filename, so an attachment
named `1` or whose filename is twelve hex characters is reachable by
ordinal and by identifier rather than by name. The number counts in the order the entities were created, which is not always the order a listing prints them in.

The number is a spelling for now and the identifier is a handle to keep.
Deleting an earlier member of a collection moves every number after it,
and the identifier an entity is born with never changes. A screen prints
the number because the number is what you are about to type, and `--json`
carries both.

## Which command takes what

Fifteen commands take a reference, and between them they accept six different sets of things. This table says what each one accepts:

| Command      | A workbench | A column | A card | Below a card | A collection |
|--------------|-------------|----------|--------|--------------|--------------|
| path         | yes         | yes      | yes    | yes          | yes          |
| edit         | yes         | yes      | yes    | yes          | no           |
| show         | no          | yes      | yes    | yes          | yes          |
| instructions | no          | yes      | yes    | no           | no           |
| attach       | yes         | yes      | yes    | yes          | no           |
| archive      | no          | yes      | yes    | yes          | no           |
| delete       | no          | yes      | yes    | yes          | no           |
| contents     | yes         | yes      | yes    | yes          | yes          |
| attachments  | yes         | yes      | yes    | yes          | yes          |
| rename       | no          | no       | no     | yes          | no           |
| cite         | no          | no       | no     | yes          | no           |
| resolve      | no          | no       | no     | yes          | no           |
| verify       | no          | no       | no     | yes          | no           |
| fail         | no          | no       | no     | yes          | no           |
| reopen       | no          | no       | no     | yes          | no           |

Six commands take a workstream: `path`, `edit`, `archive`, `delete`, `contents`, and `attachments`. The others refuse one, so the table leaves the workstream out rather than carrying a column that is mostly no.

Five of those rows carry a detail the table is too coarse to hold.
`attach` takes a comment below a card, and it takes an attachment only
with `--replace`, which replaces that attachment's bytes rather than
hanging a new file below it. It takes nothing else below a card, so `dinah
attach wb-1/questions/1 notes.md` is refused. `instructions` takes a card
or a column and nothing else at all. `contents` takes a card by the card's
own reference and never through what holds it, so `dinah contents
wb/cards/1` is refused and `dinah contents wb-1` is what you write.
`rename` takes an attachment below a card and nothing else below one, so
`dinah rename wb-1/comments/1` is refused. The checklist verbs `cite`,
`resolve`, `verify`, `fail`, and `reopen` take a checklist item and nothing
else below a card.

Each command's own help page carries the same answer for that one command,
so run `dinah help attach` when you want it beside the arguments rather
than here.

## A reference or a query

You name a thing with a reference and you find things with a query. A
reference is an address: it starts at a card, a column, the workbench, or a
workstream, and walks down to what that holds, so `wb-1/comments/1` names
one comment and `wb-1/comments` names all of them. A query asks which
cards match a condition, including conditions about what has happened to
them, and it answers with cards. If you know which thing you want, write a
reference. If you want Dinah to find the cards, write a query. Neither one
does the other's job, so a reference takes no conditions and a query names
nothing below a card.
