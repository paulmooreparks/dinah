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
carry a space, so a term naming an owner whose name has one is written like
this:

    holder:"Anne Marie"

Your shell has quoting rules of its own, and they run before Dinah sees
anything, so quote the query in whatever way leaves those inner quotation marks
intact. Inside them a backslash escapes the character after it, and the only two
characters it may come before are the quotation mark and another backslash.

An empty value asks for absence, and you write it as two quotation marks with
nothing between them. `holder:""` returns the cards nobody is holding.

## The fields you may name

Twelve fields, and no others. Seven of them describe the card as it stands now:

- `state` is the state the card is in. Give it a state's short name or its
  identifier.
- `substate` is `ready`, `active`, or `blocked`.
- `severity` is a level a workbench may declare in its own `levels:` block.
  A card carrying none is not an error, and neither is a workbench that has
  declared no severity set.
- `priority` is a level the same way, declared in its own `levels:` block.
  Severity and priority are independent, and a workbench may declare either,
  both, or neither.
- `holder` is the owner holding the card.
- `block_kind` is the class of obstacle a blocked card carries.
- `workstream` is a workstream the card belongs to. You name it by its slug or
  by its twelve-hex identifier, and never by its title. A workstream nobody has
  joined yet is a name Dinah accepts and no card matches.

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

Dinah checks every term of the query before it filters a single card, and it
tells you which word was wrong rather than returning nothing. `substate:reday`
is an error message naming `reday` and listing the three values that field
takes. `Priority>=next` is an error message saying there is no such field and
listing the twelve there are, because field names are case-sensitive and
`Priority` with a capital letter is not one of them. `severity:urgent` against
a workbench whose declared severity set does not include `urgent` is a
different error message. The field is real, and Dinah lists the severity
names it does recognize in its place. A query that is spelled correctly and
matches nothing says so plainly, so you can always tell a typo from an answer.

## Severity and priority filter, but do not rank

A card may carry a `severity` and a `priority`, each a member of a level set
its workbench declares in `workbench.md`'s `levels:` block; see the guide on
workbench layout. Query them the same way you query any other field:

    dinah query "priority:now"
    dinah query "severity:major,critical"

Both take `:` and `!=`, the same as the other equality fields, and neither
takes `>=`, `<=`, `>`, or `<`. The two axes are ranked internally, and a
workbench can declare which of its priorities outranks another, but the query
does not read that ranking. `priority>=now` is still an error message, not a
query, because the language admits no ordered comparison on anything but `at`.

A workbench that has not declared a set for an axis, or a card that carries no
value on one, is not an error. `priority:""` returns the cards carrying none.
If you want a card ranked ahead of another, sort that out with states, with
the queue order, or with a workstream, the way you always could. Severity and
priority are now things a query can name; Dinah still gives no ranked answer
to "what is ready and important."

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
