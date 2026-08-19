# Asking questions of a workbench

`dinah ls` answers a positional question, which is what one state is holding
right now. `dinah query` answers the rest. You write one string of conditions,
Dinah returns every live card that meets all of them, and the same string works
the same way from the command line and from an agent's tool call.

    dinah query "actor:alka"
    dinah query "state:doing substate:ready"
    dinah query "entered:done at>=2026-08-01"

Quote the whole query. Dinah reads it as one argument, so an unquoted query of
several words is refused with the quoted line rebuilt for you.

## The shape of a term

A query is a list of terms separated by spaces. Each term is a field, an
operator, and a value, written with nothing between them.

Every term has to hold. There is no `or` between terms, no way to negate a
whole term, and no bracketing. If you want a card that is either of two things,
say so inside one term with a comma.

    dinah query "substate:ready,active"

A comma inside one value reads as `or`, so that query returns a card in either
substate. If you want a value that has a comma in it, put the value in
quotation marks and Dinah compares it whole. Quotation marks also let a value
carry a space.

    dinah query "holder:\"Anne Marie\""

Inside quotation marks a backslash escapes the character after it, and the only
two characters it may come before are the quotation mark and another backslash.

An empty value asks for absence, and you write it as two quotation marks with
nothing between them. `holder:""` returns the cards nobody is holding.

## The fields you may name

Ten fields, and no others. Five of them describe the card as it stands now:

- `state` is the state the card is in. Give it a state's short name or its
  identifier.
- `substate` is `ready`, `active`, or `blocked`.
- `holder` is the owner holding the card.
- `block_kind` is the class of obstacle a blocked card carries.
- `workstream` is one of the workstreams the card's own list names, matched by
  membership.

The other five describe something that happened to the card, which Dinah reads
from its journal:

- `actor` is the owner who did something.
- `event` is what was done, such as `claimed`, `moved`, or `commented`.
- `entered` is the state a move carried the card into.
- `left` is the state a move carried the card out of.
- `at` is when it happened.

`at` is the one field that compares with `>=`, `<=`, `>`, and `<` rather than
with `:`, because instants rank and names do not. Write its value as a full
timestamp, `2026-08-01T09:30:00Z`, or as a date, `2026-08-01`, which Dinah
reads as midnight UTC at the start of that day. Two `at` terms give you a
window.

    dinah query "entered:doing at>=2026-08-01 at<2026-09-01"

That query returns every card that entered Doing during August, including a
card that has since moved on. It does not return a card that entered Doing in
June and was commented on in August, because the five journal fields all have
to be satisfied by one and the same recorded act.

Use `!=` to ask for the opposite of any of the nine fields that take `:`. On a
card field it means what you expect, so `state!=done` returns every card that
is not in Done. On a journal field the negation applies inside the one act, so
`actor:alka event!=commented` asks for an act by Alka that was not a comment.

## What a mistake looks like

Dinah reads the whole query before it reads a card, and it tells you which word
was wrong rather than returning nothing. `substate:reday` is an error message
naming `reday` and listing the three values that field takes. `Priority>=next`
is an error message saying there is no such field and listing the ten there
are. A query that is spelled correctly and matches nothing says so plainly, so
you can always tell a typo from an answer.

## There is no priority to filter on

The question people ask first is usually some form of "what is ready and
important". Dinah cannot answer it. A card may carry a `priority` key in its
frontmatter and Dinah will preserve it, but nothing reads it and no workbench
declares how one priority outranks another, so `priority>=next` is an error
message rather than a query. Sort out importance with states, with the queue
order, or with a workstream, until priorities are a thing Dinah understands.

## When the query cannot say it

The language is deliberately small. It does not count, average, group, join, or
search prose. When you want any of that, take the whole set and compute the
rest downstream. Two lines do it.

    dinah query --json > cards.json
    duckdb -c "select card.state_title, count(*) from (select unnest(cards) as card from read_json_auto('cards.json')) group by 1 order by 1"

`dinah query` with no query at all returns every live card, and `--json` prints
one object with the cards nested under `cards`, so a reader unnests that member
before it has rows to group. Anything that reads JSON reads that document, so
the tool you already use for numbers is the tool you use here.
