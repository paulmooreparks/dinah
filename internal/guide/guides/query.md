# Asking questions of a workbench

`dinah list <column>` answers a positional question, which is what one column
is holding right now. `dinah query` answers the rest. You write one string of conditions,
Dinah returns every live card that meets all of them, and the same string works
the same way from the command line and from an agent's tool call.

    dinah query "actor:alka"
    dinah query "column:doing state:ready"
    dinah query "entered:done at>=2026-08-01"

Quote the whole query. Dinah reads it as one argument, so an unquoted query of
several words is refused with the quoted line rebuilt for you.

## The shape of a term

A query is a list of terms separated by spaces. Each term is a field, an
operator, and a value, written with nothing between them.

Every term has to hold. There is no `or` between terms, no way to negate a
whole term, and no bracketing. If you want a card that is either of two things,
say so inside one term with a comma.

    dinah query "state:ready,active"

A comma inside one value reads as `or`, so that query returns a card in either
state. If you want a value that has a comma in it, put the value in
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

Nineteen fields are built in, and your workbench may add its own, which the
section below explains. Twelve of the built-in fields describe the card now:

- `column` is the column the card is in. Give it a column's short name or its
  identifier.
- `state` is `ready`, `active`, or `blocked`.
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
- `route` is the route the card walks, named in the workbench's own `routes:`
  block. `route:""` returns the cards walking the workbench's full column
  list, which is every card until somebody puts one on a shorter road.
- `start_after` is the first day `dinah next` and `dinah pull` may hand the
  card out, written `YYYY-MM-DD`.
- `start_by` is the last day somebody should have taken the card up.
- `due` is the last day for the card to reach a done column.
- `schedule` is what the three dates say about the card today: `overdue`,
  `late_start`, `due_soon`, `start_soon` or `not_yet`. The section on dates
  below explains each one.

The other five describe something that happened to the card, which Dinah reads
from its journal:

- `actor` is the owner who did something.
- `event` is what was done, such as `claimed`, `moved`, or `commented`.
- `entered` is the column a move carried the card into.
- `left` is the column a move carried the card out of.
- `at` is when it happened.

`at`, the three date fields and every date field your workbench declares
compare with `>=`, `<=`, `>`, and `<`, because instants and dates rank and
names do not. `at` takes those four alone. Write its value as a full
timestamp, `2026-08-01T09:30:00Z`, or as a date, `2026-08-01`, which Dinah
reads as midnight UTC at the start of that day. Two `at` terms give you a
window.

    dinah query "entered:doing at>=2026-08-01 at<2026-09-01"

That query returns every card that entered Doing during August, including a
card that has since moved on. It does not return a card that entered Doing in
June and was commented on in August, because the five journal fields all have
to be satisfied by one and the same recorded act.

The last two describe the card's checklist items:

- `item_owner` is the owner an item stores, and the empty value for an item
  that stores none.
- `item_state` is the state an item stores: `pending`, `resolved`, `verified`,
  `failed`, `waived`, or `withdrawn`.

Both item fields have to be satisfied by one and the same live item, so
`item_owner:operator item_state:pending` returns a card carrying a pending
item the operator owns, and not a card whose pending item is somebody else's
while the operator's item is resolved. A card carrying no live item is
returned by no query naming an item field.

Use `!=` to ask for the opposite of any field that takes `:`. On a card field
it means what you expect, so `column!=done` returns every card that is not in
Done. On a journal field the negation applies inside the one act, so
`actor:alka event!=commented` asks for an act by Alka that was not a comment.
On an item field it applies inside the one item, so `item_state!=pending`
returns a card carrying at least one item that is not pending.

A view's queries may write `@me` for whoever is asking, and the guide on views
explains it. `dinah query` does not expand it, so here `holder:@me` compares
against the literal text `@me`.

## What a mistake looks like

Dinah checks every term of the query before it filters a single card, and it
tells you which word was wrong rather than returning nothing. `state:reday`
is an error message naming `reday` and listing the three values that field
takes. `Priority>=next` is an error message saying there is no such field and
listing the ones there are, because field names are case-sensitive and
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
query, because the language admits an ordered comparison only on `at` and on
the date fields.

A workbench that has not declared a set for an axis, or a card that carries no
value on one, is not an error. `priority:""` returns the cards carrying none.
If you want a card ranked ahead of another, sort that out with columns, with
the queue order, or with a workstream, the way you always could. A query still
ranks nothing. For a ranked answer to "what is ready and important", run
`dinah view agenda`, which ranks the cards you can act on by severity,
priority and six other terms and shows the arithmetic behind every rank; the
guide on views describes it.

## Fields your workbench declares

If your workbench declares fields of its own in the `fields:` block of
`workbench.md`, you may name any of them in a query, provided the field
reaches cards. A wedding planner who declares `event.category` and
`vendor.deposit-required` asks for the catering and venue bookings nobody has
answered yet like this:

    dinah query "event.category:catering,venue vendor.deposit-required:\"\""

A construction foreman who declares `task.type` and `task.trade` finds the
subcontracted electrical work the same way:

    dinah query "task.type:subcontracted task.trade:electrical"

A declared field takes `:` and `!=`, the comma reads as `or`, and the empty
value asks for absence, exactly as the built-in fields work. Dinah does not
check a value against a list, so a value no card carries is a query that
matches nothing rather than an error. A field declared with type `date` also
takes the four ordered comparisons, so a planner who declares `event.date` asks
for every event from October on like this:

    dinah query "event.date>=2026-10-01"

A value on a date field has to be a date, so `event.date:2026-1-5` is an error
message rather than a query that matches nothing. A field of any other type
takes no ordered comparison.

A field the workbench does not declare is an error message that lists the
built-in fields and then the ones your workbench declares, unless some card
still carries a value under it, in which case the query finds that card. That
is the same tolerance Dinah gives a severity nobody declares any more, so a
value `dinah check` has just reported stays findable from here.

A term compares what the card stores and never asks whether the field applies
to the card. If a field's declaration says it applies only where another
field carries one of some values, a value kept on a card the declaration no
longer admits is still found by its value, which is how you find the cards
`dinah check` reports.

## Dates and what is late

A card may carry three dates, each written `YYYY-MM-DD` and each meaning the
whole of that day. `start_after` is the first day selection may hand the card
out, so `dinah next` and `dinah pull` pass over a card whose `start_after` is
still to come, and say so when that is the only work a column holds. Naming the
card with `dinah claim` still takes it up, with a warning. `start_by` is the
last day somebody should have taken the card up, and `due` is the last day for
it to reach a done column. Set them with `dinah set` or when you file the card:

    dinah add "Book the Tokyo hotel" --start-after 2026-10-06 --due 2026-10-10

Any value on a date field may be relative: `today`, or `today` with a number of
days added or taken away, such as `today+7` or `today-3`. Nothing else is
relative, so `tomorrow` is an error message. A traveller asks for everything
due in the coming week, and for every card that should already have been
started, like this:

    dinah query "due>=today due<=today+7"
    dinah query "start_by<today"

The `schedule` field asks the question directly. Dinah works out five
conditions every time it reads a card, and stores none of them. A card is
`overdue` when its due date has passed, and `late_start` when its `start_by`
date has passed and nobody has claimed it. It is `due_soon` when its due date
falls between today and the end of the workbench's soon window, and
`start_soon` when its `start_by` date does and nobody has claimed it. It is
`not_yet` when its `start_after` date is still to come. A card standing in a
done column holds none of them. A wedding planner asks for the vendors due in
the next fortnight that are not waiting on a date to start:

    dinah query "due<=today+14 schedule!=not_yet"

and a harness that wants reminders asks for everything late or close on a timer
of its own, since Dinah fires nothing:

    dinah query "schedule:overdue,late_start,due_soon"

A date field your workbench declares works the same way, so a construction
foreman who declares `task.inspection-date` finds today's inspections with:

    dinah query "task.inspection-date:today"

Today is the calendar date in the workbench's time zone, and the soon window is
seven days, unless `workbench.md` says otherwise in a block of its own:

    dinah.schedule:
      time_zone: Asia/Singapore
      soon_days: 7

`time_zone` takes a zone name such as `Europe/Berlin`, and `soon_days` a whole
number from 0 to 365. Without the block today is read in UTC, and `dinah
check` says so once a card carries a date.

## When the query cannot say it

The language is deliberately small. It does not count, average, group, join, or
search prose. When you want any of that, take the whole set and compute the
rest downstream. Two lines do it.

    dinah query --json > cards.json
    duckdb -c "select card.column_title, count(*) from (select unnest(cards) as card from read_json_auto('cards.json')) group by 1 order by 1"

`dinah query` with no query at all returns every live card, and `--json` prints
one object with the cards nested under `cards`, so a reader unnests that member
before it has rows to group. Anything that reads JSON reads that document, so
the tool you already use for numbers is the tool you use here.

## A query or a reference

`dinah query` finds cards for you, and a reference tells Dinah which one
thing you already mean. Write a reference when you know what you want and
can say where it sits: this workbench, a column, a card, or a workstream,
and on down to a comment or an attachment where the thing holding it keeps
one. A workstream keeps neither, so nothing is addressable below it. Write
a query when you want Dinah to pick the cards out by what is true of them.
The two stay separate on purpose. No condition may be written inside a
reference, and no query reaches below a card. The guide on references
teaches how one is spelled.
