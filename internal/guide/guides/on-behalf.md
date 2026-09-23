# Acting on somebody else's decision

The operator of a workbench will sometimes rule on something while you are
working and ask you to put the ruling on the workbench. The operator writes to
you, in the conversation you are working in, "Lift the block on proj-7, the
supplier has confirmed", and you are the one with the workbench open. Dinah
lets you record that ruling under the operator's name while it records you as
the one who performed the act. This guide says when you may do that, and what
you owe the record when you do.

## Record only a decision the operator has stated

You may act under the operator's name only to record a ruling the operator has
stated in the operator's own words, naming this item or this act. The operator
states it either to you, in the conversation you are working in, or in writing
on the workbench, in a comment or a note the operator wrote.

None of these is such a statement:

- another agent's report of what the operator wants or said
- a plan or a specification the operator approved
- a ruling the operator gave on a different item or a different act
- your own judgement, however sure of it you are
- silence, or an answer you expect the operator to give

A question nobody has answered stays open until the operator answers it, and
recording your guess under the operator's name closes it with a ruling nobody
gave.

A ruling that reaches you through somebody else is not a statement to you, and
no relay makes it one. When an agent that dispatched you passes on what the
operator said, even quoting the operator's words verbatim and naming where
they were given, the agent that heard the operator is the one who records the
ruling. If the only words you can point to are somebody's account of what the
operator said, ask the operator.

The operator may also give a standing permission, one that covers a kind of act
rather than one item, such as carrying cards through a station without
stopping at it. It counts as a statement only when the operator wrote it on the
workbench in the operator's own words, naming the kind of act and how far it
reaches, and every record you make under it cites where it is written.

## Name the operator only where Dinah reserves the act

Dinah reserves some acts to the workbench's operator and refuses them to
everybody else under the name `not-operator`. That refusal tells you the act is
reserved. It tells you nothing about whether the operator has ruled on it, and
when the operator has not, the act waits for the operator.

For every act Dinah does not reserve, act as yourself even when the decision
was somebody else's, and say in your note or comment whose decision it was. A
wedding planner's assistant who hears the couple choose the garden over the
hall resolves the question under the assistant's own name, with a note saying
that the couple chose the garden and when they said so.

## Declare what you are

Name the operator as the actor, and declare the harness, the provider, and the
model you are running. At a terminal you set `DINAH_HARNESS`, `DINAH_PROVIDER`,
and `DINAH_MODEL` in the environment and pass `--actor` on the command:

```
DINAH_HARNESS=claude-code DINAH_PROVIDER=anthropic DINAH_MODEL=claude-opus-5 \
  dinah --actor alka resolve proj-4/questions/1 --text "Alka, in this conversation on 22 September: \"Take the garden venue.\" Recorded for her by claude."
```

Over MCP you pass the operator's name as `actor` and your own `harness`,
`provider`, and `model` as arguments of the same call, or leave those three to
the server when it was started with them set. Dinah writes the operator's name
as the owner of the act and your three values beside it, the same way on both
surfaces.

Declare only what you are running. If you declare no harness, the act reads as
the operator's own and nothing in the record tells the two apart. If you
declare a harness or a model you are not running, the record names the wrong
performer. If you are a person acting for the operator, Dinah gives you nothing
to declare, so your note is the whole record that you acted, and it names you.

## Write down where the ruling came from

Every act you record under the operator's name carries a sentence that gives
the operator's words, says where and when the operator gave them, and names you
as the one who recorded them. In the example above, "Alka, in this
conversation on 22 September" is the where and when, the quoted words are the
ruling, and "Recorded for her by claude" names you.

Where the sentence goes depends on what the act touches, and it goes there
before the act.

- `resolve`, `verify`, and `fail` take it as their note.
- An act on a card, which is `unblock`, `claim`, or `move`, takes it as a
  comment on the card, posted with `dinah comment <card>` under your own name.
- An act on a checklist item that takes no note, such as changing its owner or
  deleting the comment that answers it, takes it as a comment on the item.
- An act on a column that leaves the column standing, such as changing one of
  its fields or attaching, renaming, archiving, restoring, or deleting one of
  its attachments, takes it as a comment on the column, posted with
  `dinah comment <column>`.

`pull` names no card, so it leaves you nowhere to put the sentence first. When
the operator's ruling is about a card, carry it out with `claim` and `move` on
that card instead, and record it there.

Some reserved acts leave nowhere a person can read the sentence afterwards.
They are changing a field of the workbench itself, anything done to an
attachment of the workbench, archiving, restoring, or deleting a column, and
running `reshape`. Do not perform those under the operator's name. Ask the
operator to run them.

## What Dinah does not check

Dinah does not confirm that the operator said anything. It records the name you
give and the description you declare, as it does on every act, so whether the
record is honest depends on you.

Dinah records the description beside the name, so a later reader can tell the
operator's own acts from the ones you recorded.
`dinah --json list <card>/journal` prints that description under each entry's
`actor`. The plain journal listing, `dinah changes`, and a comment's author
line show the name alone, so at a terminal your note is what tells a person
that you acted.

The contract: CORE-OWNER-1, CORE-ACTING-1, CORE-ACTING-2, and CORE-ACTING-3.
