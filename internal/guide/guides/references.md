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

You may write this workbench in three ways, and the three name one
workbench:

    dinah path workbench
    dinah path .
    dinah path wb/attachments

Write the third, which is your workbench's own slug, with something below
it, because Dinah reads a slug standing alone as a card and refuses it.
So `wb/attachments/1` and `workbench/attachments/1` name one file.

## A card

You write a card as its reference, which is your workbench's slug and the
card's number:

    dinah show wb-1

You may also write the card's identifier on its own:

    dinah show 4f0a1c2b8d31

You may also write the card's number on its own:

    dinah show 1

Dinah reads a bare head as a column before it reads it as a card, so a
column whose identifier, slug or title is what you typed answers instead of
the card. Write the card's reference when you want the card whatever your
columns are called.

Dinah resolves a card on its number rather than on its prefix, so a
reference carrying a prefix that names no current slug still opens the card
it named. If you rename your workbench after writing a reference down, that
reference goes on working.

## A column

You write a column as its slug, its name, or its identifier:

    dinah attach doing notes.md

## A workstream

You write a workstream as the word workstream, a slash, and the
workstream's slug or its identifier:

    dinah contents workstream/addressing

The commands that take a workstream also accept the slug or the identifier
on its own, so `dinah join wb-1 addressing` names the same workstream. You
write the prefixed form wherever a reference is read as an address, because
Dinah reads a bare handle there as a card and refuses it.

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

Dinah reads an empty collection as an empty answer rather than a mistake.
You read from the table below whether a command takes a reference of that
shape, one command at a time, because an act over a whole collection
writes an unknown number of times and nothing records how many. Restoring
its members one at a time cannot tell a reader what the act touched.

## The number and the identifier

You may write a collection member's own identifier in place of its number,
and you may write an attachment's filename in place of its number. Dinah
tries the identifier first, then the position, then the filename, so an
attachment named `1` or whose filename is twelve hex characters is
reachable by ordinal and by identifier rather than by name. The number counts in the order the entities were created, which is not always the order a listing prints them in.

You do not address a card this way. You write the card's identifier on its
own, and Dinah refuses `wb-4f0a1c2b8d31`, because a card's reference joins
your workbench's slug to the card's number and to nothing else.

Type the number and keep the identifier. Deleting an earlier member of a
collection moves every number after it, and the identifier an entity is
born with never changes. Dinah shows you the number because the number is
what you are about to type, and `dinah contents --json` and `dinah
attachments --json` give you both.

## Which command takes what

Eighteen commands take a reference, and between them they accept six different sets of things. This table says what each one accepts:

| Command      | A workbench | A column | A card | Below a card | A collection |
|--------------|-------------|----------|--------|--------------|--------------|
| path         | yes         | yes      | yes    | yes          | yes          |
| edit         | yes         | yes      | yes    | yes          | no           |
| get          | yes         | yes      | yes    | yes          | no           |
| set          | yes         | yes      | yes    | yes          | no           |
| show         | no          | yes      | yes    | yes          | yes          |
| instructions | no          | yes      | yes    | no           | no           |
| attach       | yes         | yes      | yes    | yes          | no           |
| archive      | no          | yes      | yes    | yes          | no           |
| restore      | no          | yes      | yes    | yes          | no           |
| delete       | no          | yes      | yes    | yes          | no           |
| contents     | yes         | yes      | yes    | yes          | yes          |
| attachments  | yes         | yes      | yes    | yes          | yes          |
| rename       | no          | no       | no     | yes          | no           |
| cite         | no          | no       | no     | yes          | no           |
| resolve      | no          | no       | no     | yes          | no           |
| verify       | no          | no       | no     | yes          | no           |
| fail         | no          | no       | no     | yes          | no           |
| reopen       | no          | no       | no     | yes          | no           |

Nine commands take a workstream: `path`, `edit`, `get`, `set`, `archive`, `restore`, `delete`, `contents`, and `attachments`. The others refuse one, and the table leaves the workstream out rather than carrying a column for it, so this sentence is where that answer lives.

Nine of those rows carry a detail the table is too coarse to hold.
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

## Reading the archive

`--archived` reads the archive mirror at a reference's deepest collection
step, and the live half at every step above it. `restore`, `show`, `path` and
`contents` take it. A read under the flag shows the archived half alone, where
the same flag on `dinah search` scans both halves and marks each hit. One
sentence covers both: `--archived` admits the archive mirror, and a command
that resolves a reference admits it by resolving in it, where a command that
scans a set admits it by scanning it too.

Positions under the flag count the mirror's own members. With one comment
archived and one live, `dinah show --archived wb-1/comments/1` reads the
archived one and `dinah show wb-1/comments/1` reads the live one, and neither
number moves when the other half changes.

An entity that travelled into the archive inside its holder comes back with
that holder. Archiving a card moves the card's whole directory, comments and
all, and those comments were never archived in their own right, so `dinah show
--archived wb-1/comments/1` with `wb-1` archived tells you to restore `wb-1`
rather than answering. Once the card is back, every address below it resolves
the ordinary way.

A restored column lands at the end of the column order, because the order is
what the workbench's own definition records and a restore appends to it. Run
`dinah reshape` to move it where you want it.

A reference printed under `dinah contents --archived` below the walk's root is
the address that child will have once the root is restored, and it does not
resolve while the root is archived. The listing says so on the line under the
sentence naming the root.

## Which fields a kind has

`get` and `set` name a field after the reference, and which names are legal
depends on the kind the reference resolves to. This table says what each kind
records:

| Kind | Fields |
|------|--------|
| workbench | `title`, `slug`, `operator`, `instructions` |
| column | `title`, `slug`, `kind`, `tier`, `capacity`, `hold`, `instructions` |
| card | `title`, `body`, `severity`, `priority`, `tier` |
| comment | `body` |
| item | `text`, `state`, `note`, `owner`, `column` |
| attachment | `filename`, `description` |
| workstream | `title`, `slug`, `status`, `notes` |

A field name is machine vocabulary, so it reads the same in every language and
you type it back exactly as the table spells it. Naming a field the resolved
kind does not record is refused, and the refusal lists that kind's own set, so
you can also find the answer by asking wrongly once.

The bare `dinah workbench` listing prints `title`, `slug` and `operator` and
leaves `instructions` out, because a listing that printed a whole instruction
body would stop being a listing. Read it with `dinah get workbench instructions`.

Some of these fields are the entity's prose body rather than a line of its
header: `instructions` on a workbench and on a column, `body` on a card and on
a comment, `text` on an item, and `notes` on a workstream. Those hold as many
lines as you send, and `dinah set <ref> <field> -` reads them from standard
input. Every other field holds one line, and a value carrying a line break is
refused.

A column's `hold` takes one of `on`, `off`, `out` and `both`, and nothing else.
The value says which way the column holds as well as whether it holds at all.
`dinah set <column> hold on` makes that column hold a card entering it while an
item the card carries names the column and has not been settled. `dinah set
<column> hold out` holds a card leaving it on the same terms. `dinah set
<column> hold both` holds a card either way, and `dinah set <column> hold off`
lets cards through in both directions again. `dinah get <column> hold` answers
whichever of them was written last. Writing a hold is the operator's, the way
every other write to a column is; reading it is open to anybody.

## A reference or a query

You name a thing with a reference and you find things with a query. A
reference is an address: it starts at a card, a column, the workbench, or a
workstream, and walks down to what that holds, so `wb-1/comments/1` names
one comment and `wb-1/comments` names all of them. A query asks which
cards match a condition, including conditions about what has happened to
them, and it answers with cards. If you know which thing you want, write a
reference. If you want Dinah to find the cards, write a query.
