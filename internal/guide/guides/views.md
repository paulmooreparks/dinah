# Asking the same questions every day

A view is a question you ask your workbench often enough to give it a name.
You write it once as a set of queries, and `dinah view <name>` asks all of
them again and draws each answer as a section of its own.

    dinah view
    dinah view mine
    dinah view agenda
    dinah view waiting-on-me

`dinah view` with no name lists every view you can see. Dinah ships two views.
`dinah view mine` shows the cards you hold and the cards you blocked, and
`dinah view agenda` ranks the cards you can act on by how urgent they are.
Every other view is one you or your workbench declared.

## Declaring a view

A view lives under the key `dinah.views` in the frontmatter of a file, and the
block is a mapping from each view's name to the view. This one is written for
somebody who acts as the operator:

    dinah.views:
      waiting-on-me:
        title: Waiting on me
        order: column
        sections:
          - title: At my stations
            query: "column:operator-design-review,operator-code-review,acceptance"
          - title: Questions only I can answer
            query: "item_owner:operator item_state:pending"
          - title: Blocked
            query: "state:blocked"

A view carries these members, and only `sections` is required:

- `title` is what the view's heading says. Without one, the heading is the
  view's name.
- `layout` is how the view is drawn. This build draws `list`, which is also
  what a view without one gets.
- `order` is how the cards inside each section are ordered. `arrival` is the
  order `dinah query` returns and the default. `column` orders them by where
  their column stands in the flow, and by arrival within one column. `urgency`
  ranks them, most urgent first, as the agenda below explains.
- `collapsed` lists columns by reference. It belongs to a layout this build
  does not draw yet, and a view carrying it is still read.
- `sections` lists the view's questions in the order they are drawn. Each
  section carries a `query`, written in the language the guide on queries
  teaches, or a `scope`, or both, and may carry a `title`. A section without a
  title is headed by its query, exactly as you wrote it, and a section written
  with a scope alone is headed by that scope. The one scope this build knows is
  `actionable`, the cards you can act on.

A view's name is lowercase letters and digits, joined by single hyphens and
beginning with a letter, and it runs to at most sixty-four characters.

Quote a query when it carries quotation marks of its own, and use single
quotes around it, because Dinah reads the text inside single quotes verbatim:

    query: 'holder:"Anne Marie"'

A member Dinah does not recognise is ignored and left in the file, so a view
written for a later build still draws on this one. `dinah check` names each
such member on a workbench view, which is how a misspelt `titel` gets noticed.

## Where views live

You can declare a view in three places, and a name resolves to the first place
that declares it:

1. your own settings, the `config.md` file in your user base, which only you
   see;
2. the workbench's `workbench.md`, which everybody working the workbench sees;
3. the views Dinah ships, which today are `agenda` and `mine`.

Both files take the same block, so a view copies from one to the other without
an edit. No command writes the block for you. Open `workbench.md` with
`dinah edit workbench`, and `config.md` in any editor.

The listing prints one row for every declaration, including the ones a nearer
declaration hides. The first row of each name is marked as the one a draw
uses. A workbench or a user replaces a shipped view by declaring a view of the
same name; there is no way to remove one without replacing it.

A view that cannot be drawn keeps its place. If your own `waiting-on-me` is
malformed, `dinah view waiting-on-me` refuses and names what is wrong, rather
than quietly drawing the workbench's `waiting-on-me` in its place. The listing
marks every malformed view with the reason.

A file Dinah cannot read is treated the same way. If your `config.md` exists
and cannot be read, or its `dinah.views` value is not a mapping, `dinah view`
refuses rather than drawing views that file might have replaced. The same
holds for a `workbench.md` whose `dinah.views` value is not a mapping. A
`config.md` that does not exist is simply an empty layer.

## Asking about yourself

`@me` stands for you in a view's query. `holder:@me` asks for the cards you
hold, whoever you are, so one view serves everybody who reads it.

`@me` is replaced only where it is a whole value written without quotation
marks after `:` or `!=`. `holder:x,@me` asks for the cards `x` holds or you
hold. `holder:"@me"` asks for an owner literally called `@me`, which is how you
ask about one, and `holder:@meg` asks about an owner called `@meg`. Your name
is compared as one value, so a name with a comma or a space in it still
matches.

A view whose queries use `@me` needs to know who you are, and without an actor
Dinah refuses to draw it. A view that never uses `@me` draws for anybody.

`@me` belongs to views. `dinah query`, `dinah tree` and `dinah search` compare
it as the literal text `@me`, which on an ordinary workbench matches nothing.

## Asking about checklist items

Two query fields describe a card's checklist items rather than the card
itself, `item_owner` and `item_state`, and the guide on queries lists what
each one reads. Any query may name them, and a view is where they are most
often wanted.

Every item term in one query has to be satisfied by one and the same item.
`item_owner:operator item_state:pending` finds a card carrying a pending item
owned by the operator, and it does not find a card whose pending item belongs
to somebody else while an item owned by the operator is already resolved.

`!=` applies inside that one item, so `item_state!=pending` finds a card
carrying at least one item that is not pending. A card with no live items has
nothing to satisfy an item term with, so neither `item_state:pending` nor
`item_state!=pending` finds it.

`item_owner:operator` finds only items that store the operator as their
owner. An item that stores no owner is found with `item_owner:""`.

## What a drawn view shows

The heading names the view and the actor its questions were asked as, and
says so when that actor is the workbench's operator. Each section follows with
its title and the number of cards it found, then a table of those cards. A
section whose query names an item field adds an Item column naming the item
that matched; the Holder column appears only when somebody other than you
holds one of the cards, and the Pri and Sev columns only when a card carries
that level. When your terminal's width is known, a long title is cut to fit
and ends in an ellipsis. Piped output keeps every title whole.

A card that matches two sections appears in both, and each section counts it.

`dinah view --json` prints the same answer as one object, and the MCP tool
`view` answers with the identical object.

## The agenda: what needs you first

`dinah view agenda` ranks the cards you can act on, most urgent first, and
shows the arithmetic behind every rank.

    dinah view agenda
    dinah view agenda --explain
    dinah view agenda --explain dinah-572

Which cards you can act on depends on who you are. If you are the
workbench's operator, your agenda holds these cards, each once:

- every card standing in a column you own, unless that column is a done
  column;
- every card carrying a pending open question or decision that is yours to
  answer, meaning one owned by the operator or owned by nobody, in any column;
- every blocked card;
- every card `dinah next` would offer you.

If you are anybody else, your agenda holds exactly the cards `dinah next` would
offer you, which is at most one card per column. A card above your tier, a card
behind the head of its queue and a card somebody else holds are not in it.

A card's urgency is the sum of eight terms, each worth a number of points:

| Term | Default | What earns it |
|---|---|---|
| waits on you | 10 | the card stands in a column you own that is not a done column; only the operator owns columns |
| your question | 4 per item, at most 3 items | a pending open question or decision on the card is yours to answer |
| priority | 0, 2, 4, 6 | the card's priority, lowest level first |
| severity | 0, 1, 2, 4 | the card's severity, lowest level first |
| blocked | 3 | the card is blocked |
| blocks others | 2 per card | nothing yet, because Dinah does not read `blocks` links yet |
| age | 0.5 per whole day, at most 5 days | the whole days the card has stood in its current column |
| stale claim | 3 | the card is ready and its last claim lapsed rather than being released |

An item is yours when you are the operator and it waits on the operator, when
its owner is `holder` and you hold the card or would hold it by taking the card
`dinah next` offers you, or when its owner is your own name. Acceptance
criteria never count, and neither does an item that is not pending.

A card with no priority scores nothing on priority, rather than scoring as the
lowest level, and so does a card whose priority your workbench does not
declare. The weights line up with the top of the levels your workbench
declares, so the highest level always takes the last weight. With the default
weights, a workbench declaring three priorities scores them 2, 4 and 6, and one
declaring five scores them 0, 0, 2, 4 and 6.

A lapsed claim reaches people unevenly. An agent meets the stale claim term only
on a card `dinah next` already offers it, where the term moves that card up
among the column heads. The operator meets it on any card his agenda holds. So
a lapsed card missing from somebody's agenda is not evidence that nothing is
stale.

Cards with equal urgency are ranked in the order the queue itself uses: by
when they arrived in their current column, earliest first, and then by card
number, lowest first.

### Changing the weights

Your workbench declares its own weights in `workbench.md`, under the key
`dinah.urgency`. Every term you leave out keeps its default, and so does every
member of `your-question` and `age` you leave out:

    dinah.urgency:
      waits-on-you: 10
      # priority weights, lowest first: later, soon, next, now
      priority: [0, 2, 4, 6]
      age:
        per-day: 0.5
        cap: 5

The keys are `waits-on-you`, `your-question` with `per-item` and `cap`,
`priority`, `severity`, `blocked`, `blocks-others`, `age` with `per-day` and
`cap`, and `stale-claim`. A weight is a number from -1000 to 1000 written with
at most one digit after the decimal point, so `0.5` works and `0.25` and `1e1`
do not. A negative weight pushes a card down. A cap is a whole number from 0 to
1000. Dinah reads what you write by these rules:

- Write a weight bare, as `blocked: 3`. Dinah reads `blocked: "3"` as text and
  refuses it.
- Write a list on one line, as `priority: [0, 2, 4, 6]`. Dinah reads a list
  written one entry per line as text and refuses it.
- Write `your-question` and `age` with one member per line beneath them. Dinah
  refuses `age: {per-day: 0.5, cap: 5}`.
- Put a comment on a line of its own. A comment after a value becomes part of
  the value, and Dinah refuses it.

If you write `dinah.urgency:` with nothing beneath it, or only comments, Dinah
uses every default. Only the workbench's weights count, and a `dinah.urgency`
block in your own `config.md` has no effect.

If Dinah cannot read the block, every view ordered by urgency refuses with
`dinah.malformed-urgency`, naming the term and the value it read, rather than
ranking on weights nobody declared. Every other view still draws. `dinah
check` reports the block, reports a member it does not know such as a misspelt
`age.perday`, and reports a priority or severity list whose length differs from
the levels your workbench declares.

### Ranking any view

`order: urgency` works on any view, including one with several sections. Dinah
ranks each section on its own and numbers it from 1, and a card in two sections
carries the same urgency in both. A view ordered by urgency needs to know who
you are, and so does a view with a section scoped to `actionable`.

`scope: actionable` on a section selects the cards you can act on, the same
set the agenda holds. A section carrying both a scope and a query holds the
cards that match the query and are in the scope, so this agenda splits your
work in two:

    dinah.views:
      agenda:
        title: What needs me first
        order: urgency
        sections:
          - title: Terminal work I can take
            scope: actionable
            query: "workstream:terminal"
          - title: Everything else I can take
            scope: actionable
            query: "workstream!=terminal"

### Reading a rank

In a view ordered by urgency, each section draws the columns Rank, Card,
Column, Item, Holder, Urgency, Why and Title, and leaves out Item and Holder as
it does in any view. The Urgency figure always carries one digit after the
decimal point. The Why column names every term that moved the card, in the
order of the table above, including one whose weight is negative. It names a
level by the level itself.

`--explain` prints every term behind a rank. With a card, Dinah prints all
eight terms of that card, one per line, the ones worth nothing included, then
a rule and the total. Without a card, Dinah prints the same for every card in
rank order, under each section's heading. `--explain` on a view ordered any
other way refuses with `dinah.view-not-ranked`, because such a view has no
arithmetic to show.

    $ dinah view agenda --explain dinah-572
    dinah-572: dinah setup connects a named harness to a workbench
      waits on you   Acceptance is yours               +10.0
      your question  none of its items is yours        +0.0
      priority       now, rank 4 of 4                  +6.0
      severity       major, rank 3 of 4                +2.0
      blocked        not blocked                       +0.0
      blocks others  blocks links are not read yet     +0.0
      age            1 day in Acceptance, 0.5 per day  +0.5
      stale claim    no lapsed claim                   +0.0
      -------------  --------------------------------  ------
      urgency        the sum of the terms above        18.5

A card after the view's name narrows the view to that card. Every section
holding it draws its row alone, with the rank the card holds in the whole
section, and every other section draws empty. A card no section selects
refuses with `dinah.card-not-in-view`.

The ranked table, `--explain` and a card after the view's name are described
here for the list layout, which is the one layout this build draws.

`dinah view agenda --json` carries each ranked card's rank and score under
`urgency`, and with `--explain` every term with the values it was computed
from. The MCP tool `view` answers with the identical object.

## A section this workbench cannot ask

A view in your own settings is read on every workbench you open, and a column
one workbench has is often missing on another. So a section whose query names
something this workbench lacks is drawn as a section that was not asked,
saying why, and the rest of the view draws as usual. That covers a column, a
workstream, a severity or priority level, a route, and a field the workbench
declares none of.

Any other mistake stops the whole view, because it is wrong on every
workbench. That covers a misspelt query, a field Dinah does not have, a state
that does not exist, and `@me` with nobody to stand for. Dinah names the
mistake first, and then the view and the section it came from.

`dinah check` reports a workbench view whose query the workbench refuses, so
its owner meets a stale column name without drawing the view.

## The views Dinah ships

`agenda` is titled What needs me first, ranks by urgency, and has one section,
Cards you can act on, scoped to `actionable`. The agenda section above
describes it.

`mine` is titled My cards, orders its cards by column, and asks two things.
Claimed by me is `holder:@me`, the cards you hold. Blocked by me is
`state:blocked actor:@me event:blocked`, the blocked cards on which you
recorded a block.

Blocking a card clears whoever held it, which is why the second section reads
the journal rather than the holder. It also finds a card you blocked without
ever holding it, which is intended: the obstacle is one you raised and yours
to chase. It can find one more kind of card, one you blocked that was later
unblocked and blocked again by somebody else, because a query cannot ask which
block is the current one.

To change `mine` or `agenda`, declare your own view of the same name, in your
settings or on the workbench, and yours is drawn in its place. A replacement
agenda keeps the agenda's selection by writing `scope: actionable`.
