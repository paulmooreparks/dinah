# References

You name a thing to Dinah by writing a reference. A reference names this
workbench, a state, a card, or something that hangs off a card, and you write
it as a path with slashes between its parts.

## This workbench

You may write this workbench in two ways, and the two mean the same thing:

    dinah path workbench
    dinah path .

## A card

You write a card as its reference, which is your workbench's slug and the
card's number:

    dinah show wb-1

## A state

You write a state as its slug, its name, or its identifier:

    dinah attach doing notes.md

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

You may write three shorter spellings that select one kind of checklist item:

    dinah path wb-1/oq               the open questions
    dinah path wb-1/ac               the acceptance criteria
    dinah path wb-1/d                the decisions

You may write an entity's own identifier in place of its number. The number
counts in the order the entities were created, which is not always the order a
listing prints them in.

If the collection you name holds nothing, Dinah tells you that nothing answers to the reference rather than telling you the collection is empty.

## Which command takes what

Seven commands take a reference, and no two of them take quite the same set of
things. This table says what each one accepts:

| Command      | This workbench | A state | A card | Below a card |
|--------------|----------------|---------|--------|--------------|
| path         | yes            | no      | yes    | yes          |
| edit         | yes            | no      | yes    | yes          |
| show         | no             | no      | yes    | yes          |
| instructions | no             | yes     | yes    | no           |
| attach       | yes            | yes     | yes    | yes          |
| archive      | no             | yes     | yes    | yes          |
| delete       | no             | yes     | yes    | yes          |

Two of those rows carry a detail the table is too coarse to hold. `attach`
takes a comment or an attachment below a card and takes nothing else below one,
so `dinah attach wb-1/journal notes.md` is refused. `instructions` takes a card
or a state and nothing else at all.

Each command's own help page carries the same answer for that one command, so
run `dinah help attach` when you want it beside the arguments rather than here.
