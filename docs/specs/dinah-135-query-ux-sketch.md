# dinah-135: the three questions, written out under each candidate grammar

This sketch exists so the grammar ruling is made against syntax somebody can
read rather than against adjectives. The card names four candidate shapes for
the query language. Each section below writes the same three questions as real
command lines under one candidate, and then says what that candidate costs a
second implementer of the contract.

Nothing here is implemented. Every `dinah query` block is a sketch of intended
output. Every block introduced as real output came out of a binary built from
the trunk head in a scratch tree, and is marked as such.

## The fixture

The blocks below all read one workbench, in a directory named
`dinah-135-play`, holding five cards in three states. Alka holds one card, Paul
holds another and has blocked a third, and one card has already reached Done.

Real output, from the binary:

```
  Card  Standing  Title
  ----  --------  ---------------------
  q-1   active    Book the caterer
  q-2   ready     Choose the flowers
  q-3   active    Send the invitations
  q-4   ready     Hire the photographer
  q-5   blocked   Confirm the venue
```

That is `dinah ls` with no argument. Notice that it does not tell you which
state any card is in, because it was built for the single-state case and prints
the state nowhere. Every candidate below has to fix that much.

## The three questions

The card names three questions a listing verb cannot answer. Stated as a person
would ask them:

1. Which cards has Alka touched?
2. What is ready above a given priority?
3. Which cards entered Doing during August?

Question 2 is the interesting one, and every candidate fails it for the same
reason. Dinah stores no priority. A card may carry a `priority:` key in its
frontmatter, and the tool preserves it untouched, but no code reads it and no
workbench declares what the values mean or how they rank. `docs/design/format.md`
designs a `levels:` block in the workbench definition that would supply the
ranking, and nothing implements it. The grammar therefore has to be able to
express question 2 before Dinah can answer it, and answering it is a separate
piece of work.

## Candidate A: a subset of SQL

```
dinah query "select * from cards where id in (select card from events where actor = 'alka')"
dinah query "select * from cards where substate = 'ready' and priority >= 'next'"
dinah query "select * from cards where id in (select card from events where to_state = 'doing' and ts >= '2026-08-01' and ts < '2026-09-01')"
```

Question 1 and question 3 both need a subquery, because the fact being filtered
on lives in the card's journal rather than on the card. You could flatten that
with a join instead, and then a reader has to know which of the two spellings
this subset accepts.

What it costs a second implementer: the whole dialect, pinned normatively. The
profile would have to say which SQL this is, down to whether `>=` on a string
compares by code point or by a declared rank, whether `in` accepts a subquery,
and what happens to `null`. Leave any of that unsaid and the contract quietly
becomes whatever one SQLite build accepts, which is the git and libgit2 failure
restated. There is a second cost that no amount of specification removes: a
reader who knows SQL reads every missing feature as a bug rather than as a
boundary.

## Candidate B: a language of Dinah's own

```
dinah query "touched by alka"
dinah query "ready and priority at least next"
dinah query "entered doing between 2026-08-01 and 2026-09-01"
```

This reads best of the four, and it is the most expensive thing on the list. It
needs a grammar, a parser, error messages that point at a column, documentation,
a completion story for the LSP, and an answer to every "why can it not say X"
for as long as the tool exists. A second implementer owes all of it, exactly.

## Candidate C: a qualifier grammar, which is what this spec recommends

```
dinah query "actor:alka"
dinah query "substate:ready priority>=next"
dinah query "entered:doing at>=2026-08-01 at<2026-09-01"
```

Every term is `field`, an operator, and a value. Terms combine with and, and
nothing else. A value may carry commas, which read as or within that one term,
so `substate:ready,active` asks for either. Quoting a value turns off the comma
split and lets a value carry a space.

The shape is the one GitHub and Jira search already use, so most people who meet
it have used it before and agents write it fluently with no served guidance at
all. It is small enough to write down completely, which matters more than any of
that.

Sketch of the intended output for question 1:

```
$ dinah query "actor:alka"
  Card  State   Standing  Title
  ----  ------  --------  ---------------------
  q-3   Doing   active    Send the invitations
  q-4   Doing   ready     Hire the photographer
```

Sketch of the intended output for question 3:

```
$ dinah query "entered:doing at>=2026-08-01 at<2026-09-01"
  Card  State   Standing  Title
  ----  ------  --------  ---------------------
  q-1   Done    active    Book the caterer
  q-3   Doing   active    Send the invitations
  q-4   Doing   ready     Hire the photographer
```

Card q-1 is in the answer because it passed through Doing during the period, and
it now sits in Done. The question asks about what happened, so the answer is not
restricted to where the cards ended up.

Sketch of what question 2 does today, before any level set exists:

```
$ dinah query "substate:ready priority>=next"
Dinah knows no field called priority.
```

What it costs a second implementer: the field list, the six operators, the
splitting rules, and one paragraph fixing how the journal-derived fields combine.
That last paragraph is the only subtle part, and section 4 of the spec states it.

## Candidate D: the same grammar, with analytics delegated to the export

Candidate D is not a rival to candidate C. It is the answer to the question
candidate C invites, which is what somebody does when the grammar cannot say
what they want.

The answer is that they ask for the set and compute the rest themselves:

```
$ dinah query --json > cards.json
$ duckdb -c "select card.state_title, count(*) from (select unnest(cards) as card from read_json_auto('cards.json')) group by 1 order by 1"
Doing  2
Done   1
```

`--json` emits one object rather than a stream of cards, and the cards sit under
`cards`, so the second line unnests that member before it has rows to group. The
DuckDB line above was run against a document written by hand in the output shape
the spec's section 7 fixes, because `dinah query` does not exist yet; the
`dinah query` line above it is a sketch.

`dinah query` with no query and `--json` emits every live card in the canonical
form. Anything the grammar cannot express, and that includes every aggregate,
every average, every count by group, and every join to somebody else's data, is
one of these two lines away. Nothing in the contract grows to make it possible,
because the canonical card form is already frozen and DuckDB reads it natively.

## The help text, under candidate C

```
query [<query>] [--json]

the cards of this workbench that match a query

What can go wrong, in the order each is checked:
  Order  What can go wrong                                 Refusal
  -----  ------------------------------------------------  ---------------------
  1      every term of the query parses                    malformed
  2      every field named is one this tool knows          dinah.unknown-field
  3      every operator is one the named field accepts     dinah.unknown-field
  4      each substate and event value is a legal one      dinah.unknown-value
  5      each state named is one the workbench declares    unknown-state
  6      each workstream named is one a live card lists    dinah.unknown-value

Exit codes: 0 ok, 2 refused, 3 stale, 4 unreachable.
```

## What the ruling decides beyond the syntax

Whichever candidate wins, one further question stays open, and section 2 of the
spec puts it to the operator. A grammar can ship as Dinah's own tool surface, the
way `ls`, `status`, and `log` already do, or it can enter the shared contract so
that a second tool has to parse it identically. This spec recommends the first.
The core profile specifies no read verb at all beyond history and instruction
serving, and its boundary table puts measurement over a workbench's history
outside the contract. Five of the ten fields read a card's recorded history, so
admitting the grammar whole reopens that boundary row. Ranked priority is out of
the first cut on its own account and stays out whichever way the operator rules,
so it is not an argument for either answer.
