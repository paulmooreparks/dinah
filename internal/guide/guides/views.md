# Asking the same questions every day

A view is a question you ask your workbench often enough to give it a name.
You write it once as a set of queries, and `dinah view <name>` asks all of
them again and draws each answer as a section of its own.

    dinah view
    dinah view mine
    dinah view agenda
    dinah view board
    dinah view waiting-on-me

`dinah view` with no name lists every view you can see. Dinah ships three
views. `dinah view mine` shows the cards you hold and the cards you blocked,
`dinah view agenda` ranks the cards you can act on by how urgent they are, and
`dinah view board` draws every live card of the workbench as columns side by
side. Every other view is one you or your workbench declared.

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
- `layout` is how the view is drawn. `list`, which a view without one gets,
  draws each section as a table of cards. `columns` draws each section as a
  board, which the section on the board below describes.
- `order` is how the cards inside each section are ordered. `arrival` is the
  order `dinah query` returns and the default. `column` orders them by where
  their column stands in the flow, and by arrival within one column. `urgency`
  ranks them, most urgent first, as the agenda below explains.
- `collapsed` lists the columns a `columns` view draws as a count on one line
  rather than as a column, each named by reference. A `columns` view without
  it collapses the intake and done columns, and `collapsed: []` collapses
  nothing. It has no effect on the `list` layout, and Dinah ignores an entry
  naming a column this workbench does not have.
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
3. the views Dinah ships, which today are `agenda`, `board`, and `mine`.

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

The ranked table belongs to the list layout. A `columns` view ordered by
urgency draws a board instead, with each column's cards in urgency order, and
`--explain` and a card after the view's name work on it as the section on the
board below describes.

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

`board` is titled Board, uses the `columns` layout, orders its cards by
column, and asks one question, `state:ready,active,blocked`, which matches
every live card on any workbench. It declares no `collapsed` member, so it
counts the cards in the intake and done columns rather than drawing them.

To change `mine`, `agenda`, or `board`, declare your own view of the same name,
in your settings or on the workbench, and yours is drawn in its place. A
replacement agenda keeps the agenda's selection by writing `scope: actionable`.
To draw the board with a shorter command, give yourself an alias named board:

    dinah config set alias.board "view board"

## The board

When a view uses the `columns` layout, Dinah draws each section as columns
side by side, each headed by its title and the number of cards it holds, with
two lines for every card beneath:

    Triage (3)                 Design Queue (9)           Agent Design Review (1)
    ─────────────────────────  ─────────────────────────  ─────────────────────────
    ○ 4  later                 ○ 565 ◆  next              ● 598  claude  now
      Jira-resolution workfl…    Dinah's 1.0 scope is w…    A starved run updates …

A card's first line shows its state, its number, its holder and its priority.
The mark says whether the card is ready (`○`), held (`●`) or blocked (`✕`).
The number is the card's reference without the workbench's slug, and
`dinah show 565` finds it. A held card names its holder and a blocked card
names the kind of block. The second line is the card's title. When a column
is too narrow for everything, Dinah drops the priority first, then shortens
the holder, then drops it, and it never drops the mark, the number or the
title. The board never shows severity, which `dinah show` does.

`◆` marks a column owned by the operator, and a card with a checklist item
waiting on the operator.

Dinah leaves out any column that holds none of a section's cards. It draws as
many columns side by side as fit at twenty characters or more each, and it
starts a new row of columns underneath when the window runs out. A card
standing in a column the workbench no longer lists is drawn in a column of its
own at the end, titled by its identifier.

Each column shows at most five cards and then a line such as `+4 more`.
`dinah view board --all` shows every card. The drawing uses one column fewer
than your window, so no line ever reaches its right edge.

The layout decides how a view is drawn, and the order only decides where each
card stands. So a `columns` view ordered by urgency lists each column's cards
most urgent first and draws no rank, score, or Why text. `--explain` prints
the agenda's blocks in place of the board, as it does for any view ordered by
urgency. A card after the view's name narrows the board to that card. Each
section holding it draws one column, the card's own, with a count of 1, even
when the view collapses that column, and every other section draws that
nothing matches.

### Marks, colour, and plain characters

Dinah draws the board with the Unicode symbols above unless you ask for plain
characters. `--plain` draws one board with `o`, `*`, `x` and `!` for the four
marks, `-` for the rules, and `...` where it shortens text. To use plain
characters every time, set

    dinah config set glyphs plain

and `dinah config set glyphs` goes back to the symbols. Dinah cannot find out
whether your console's font has the symbols, so it never guesses. If you see
boxes or question marks where the marks should be, use `--plain`.

Some terminals, especially ones set up for Chinese, Japanese or Korean, draw
`○`, `●`, `◆`, `─` and `…` two columns wide instead of one. The board then
drifts out of line, and under `--watch` a long rule can wrap onto the next
row and scroll the whole window. Use `--plain` on such a terminal.

On a terminal, Dinah colours the marks: blue for held, red for blocked and
yellow for waiting on the operator. It colours only the marks and never a
title, and every state keeps its own mark, so nothing depends on seeing the
colour. If the `NO_COLOR` environment variable is set to anything other than
an empty string, Dinah draws no colour at all and changes nothing else, so the
marks stay as they are.

### Files, pipes, and the machine form

If you redirect the board to a file or a pipe, Dinah draws the same board
without colour, as wide as `COLUMNS` says or 80 columns wide when nothing
says, so the file is the same on every machine:

    COLUMNS=160 dinah view board > board.txt

The board shortens titles to fit its columns. If you need every title whole,
use `dinah view board --json`, which answers with every card of every section,
never capped, and names the collapsed columns in a `collapsed` member.

## Watching a view

`dinah view <name> --watch` draws the view and then draws it again, in place,
each time the workbench changes, until you press Ctrl+C. It works with any
view, the `list` layout included:

    dinah view board --watch

The bottom row says when Dinah last drew the view and what changed, such as
`updated 09:41:07 · dinah-598 moved Spec to Agent Design Review by claude ·
Ctrl+C stops`. Dinah also draws the view again when you resize the window,
within about a second, and when a claim on a card it drew expires. When the
view is taller than the window, Dinah draws what fits and says how many lines
it left out.

Dinah refuses `--watch` together with `--json`, without a view name, and when
your output is not a terminal. It also refuses a window narrower than 40
columns or shorter than 6 rows, and a Linux or macOS terminal whose
description, named by `TERM`, it cannot find or that lacks what redrawing
needs. If you shrink the window below 40 by 6 while a watch is running, Dinah
says so on the first row and draws the view again once the window is big
enough.

A watch reads the window's size from `COLUMNS` and `LINES` when they are set,
and from the terminal otherwise. If you exported either one and have since
resized the window, the watch draws for the old size, and rows wider than the
window wrap and overwrite each other. Unset them, or set them to the window's
size, before you start a watch.

On Linux and macOS, Dinah reads the terminal's description, named by `TERM`,
from the places ncurses searches: the directory `TERMINFO` names, then
`~/.terminfo`, then each directory `TERMINFO_DIRS` lists, then
`/etc/terminfo`, `/lib/terminfo` and `/usr/share/terminfo`. That last list is
the one Ubuntu 24.04 documents; a system built with another one may keep its
database elsewhere. If Dinah finds no description, a watch is refused with
`no-terminal-description` and the board is drawn without colour. `infocmp -D`
prints where your system keeps its database, and setting `TERMINFO_DIRS` to
that directory fixes both.

A watch reads your workbench exactly as `dinah view` does. Like any read, it
records a claim that has expired, under the name of whoever held it.

Some things a watch does not do:

- It does not read the keyboard. Anything you type is echoed as your terminal
  echoes it, and the next redraw covers it.
- It does not handle Ctrl+Z. If you suspend a watch in a POSIX shell, the
  cursor stays hidden until the watch resumes or another program shows it.
- It cannot restore your terminal if it is killed rather than interrupted, or
  if you close a Windows console while it runs. The cursor may stay hidden,
  and the text colour changed, until the next program resets them.

When you press Ctrl+C, Dinah puts the colour and the cursor back, leaves the
last drawing on the screen, and returns you to your prompt below it.

## Working a view from the keyboard

`dinah tui` draws a view and lets you work it with keys. You move between
cards, read one in full, and claim, move, release or comment on it without
leaving the screen. You can answer, verify or withdraw the questions and
criteria a card carries, reach every other act on a card from one menu, and
type any other command at a command line inside the screen. With no view
named, Dinah works the board, so you need no alias for that; name any view
`dinah view` lists to work that one instead:

    dinah tui
    dinah tui agenda

Dinah runs the interface only when both your input and your output are a
terminal. If either is a file, a pipe or a program such as an MCP host, or
the window is smaller than 60 by 12, Dinah refuses with
`dinah.tui-unavailable` before it draws anything. `dinah view <name>` draws
the same view once and works anywhere.

The interface is a program of its own, `dinah-tui`, which the install scripts
put beside `dinah`, and `dinah tui` starts it. The library the interface is
drawn with costs every program that links it at start-up, about 26 ms on
Windows and a 2.2 MB table on every platform, so it lives in its own program
and `dinah` itself pays none of it. `dinah tui` looks for `dinah-tui` beside
`dinah`, then beside the file a link to `dinah` points at, then on your
`PATH`, and refuses with `dinah.tui-missing` when it finds none. `dinah-tui`
refuses with `dinah.tui-skew` when the `dinah` that started it comes from
another build, because the two read the same workbenches; install both from
one release, as the install scripts do.

You can also run `dinah-tui` yourself, and it reads its words as those of
`dinah tui`, so `dinah-tui agenda` works the agenda. The one exception is
`dinah-tui version`, which answers as `dinah version` does, so you can ask
which build it is. To work a view named `version`, run `dinah tui version`.

### What the screen shows

The first row names the workbench, the view and any filter you set, and on
the right who you are acting as. The second row lists the view's lanes, one
per column that holds a card, with the lane you are in between brackets; the
screen shows one lane at a time. Below the rule, the cards of that lane fill
the left of the window, and in a window at least 100 columns wide the
selected card's detail fills the right, exactly as `dinah show <card>
--fields card,body,checklist --unresolved` prints it, so the questions,
decisions and criteria still open on the card are listed under its body. The row above the last says when Dinah last
read the workbench and what changed, and shows the answer to anything you
just did. The last row lists the keys you can press, and only those.

### The keys

While you are looking at a lane:

- Up and Down, or `k` and `j`, select the previous or the next card.
- Left and Right, or `h` and `l`, move to the previous or the next lane.
- Page Up and Page Down move a screenful; Home and End go to the first or
  the last card.
- Enter opens the selected card in full, and Enter or Backspace closes it.
- `t` claims the card, `r` releases it, and `c` opens a prompt for a comment.
- `i` opens item mode over the card, which lists its questions, decisions and
  criteria that carry an act you may take. The footer offers `i` only when
  one does.
- `x` opens the actions menu, which lists every act the workbench would
  accept from you on the card, such as blocking it, filing an item on it,
  linking it or setting a field on it.
- `a` moves the card to the next column on its route, which the footer names
  accept when that column is a done column and advance otherwise. `b` sends
  it back to the column its column rejects to. `m` opens a menu of every
  column you may move it to, where the arrows or `j` and `k` pick a row, a
  digit chooses that row, Enter chooses the highlighted one, and Ctrl+G or
  `q` closes the menu.
- `/` filters the view with a query, and `:` opens the command line, where
  you can type any command or the name of a view, a card or a column to jump
  to it.
- `>` runs `dinah next`, `S` runs `dinah status`, `F` asks for a phrase and
  runs `dinah search` with it, `C` runs `dinah changes` for the selected card,
  and `W` runs `dinah whoami`.
- `?` shows every key, Ctrl+L draws the whole screen again, and `q` or
  Ctrl+C quits.

In the command line and the filter prompt, Enter carries out what you typed
and Ctrl+G closes the prompt without doing anything. The comment prompt, and
every prompt that asks for an answer or a reason, holds several lines: Enter
starts a new line, Ctrl+D posts what you wrote, and Ctrl+G closes the prompt
without posting. Ctrl+C quits from anywhere, without doing the thing a menu or
a prompt was open for.

In item mode, the arrows or `k` and `j` highlight an item, and Page Up, Page
Down, Home and End move the highlight further. A letter acts on the
highlighted item: `r` answers a question or a decision, `v` verifies a
criterion, `f` fails it, `w` waives an item, `x` withdraws it, `o` reopens it,
and `c` records evidence on it. The footer lists only the letters the
workbench would accept on the highlighted item, and a letter it does not list
does nothing. These letters mean something else outside item mode, and the
footer says what each does where you are. Ctrl+G or `q` closes item mode.

The actions menu numbers its rows as the move menu does. Choosing a row asks
for whatever the act needs, from a menu of the values the workbench accepts
where there is a closed set, such as the fields you may set or the
workstreams the card is not in yet, and from a prompt where you type the
value. Choosing "edit it in your editor" hands the terminal to your editor,
exactly as `dinah edit` does, and takes it back when the editor ends.

Esc does nothing in the interface. On Linux and macOS it also swallows the
printable key you type after it, and Esc followed by `[` or `O` swallows the
next few keys, up to and including the first letter or `~`, because the
keys your terminal sends for the arrows begin that way. Esc followed by
Ctrl+C still quits.

### What you are offered

The footer offers only the acts the workbench would accept from you on the
selected card, so an agent sees no move out of a column the operator owns,
and nobody sees a claim the card's tier refuses. Each act is checked again
when you press its key: if the card changed after Dinah drew it, the act is
answered stale and changes nothing, and Dinah reads the view again. Dinah
records each act in the card's journal exactly as the same act typed on the
command line records it, with your name and whatever your environment
declares about the model you run on.

Item mode and the actions menu follow the same rule. Only the operator is
offered an answer, a verification or a failure of an item the operator owns,
a waiver of any item, and the reopening of a failed or waived item or of one
the operator owns. Nobody but the operator is offered the withdrawal of an
acceptance criterion, unless the card carries the criterion-retirement grant
and the criterion has not failed or been waived. On a workbench that declares
evidence, a criterion with no citation offers `c` and neither `v` nor `f`,
until you record evidence on it. Answers, verifications and the other acts on
an item take no revision, because Dinah checks the item's own state when you
act, so a comment somebody posts on the card meanwhile does not make your
answer stale.

For example, to walk the cards waiting in Acceptance as the operator, press
`l` until the Acceptance lane is between brackets, press Enter to read the
selected card, and press `a` to accept it. Dinah moves it to Done and selects
the next card in the lane.

### The command line

The command line on `:` runs any command `dinah` runs, inside the screen and
in the same process, and draws the view again when the command ends. Type the
command as you would at a shell, with or without `dinah` in front of it:

    : list columns
    : file dinah-12 decision "Use the stream reader"

Dinah reads your aliases from your settings, so an alias you set with
`dinah config set alias.<name>` works here too. A line whose first word names
neither a command nor an alias is a jump: Dinah looks the whole line up as a
view, then a live card, then a column, then a card in the archive, and goes
there. To reach a view, a card or a column whose name is also a command, write
`@` in front of it, as in `:@status` for a view named `status`. If you jump
to an archived card, Dinah opens it in full, and its actions menu offers to
restore it.

Tab completes the word at the end of the line, as a shell's Tab does. If one
word fits, Dinah puts it in with a space after it, and if several fit, Dinah
fills in what they share, and a second Tab lists them above the line. Tab
does not complete the names of files or directories, since the screen offers
no way to browse them.

What a command writes appears in the row above the footer when it is three
lines or fewer and each fits the window. Anything longer opens output mode,
which fills the screen with what the command wrote. The arrows or `k` and `j`
scroll it, Page Up, Page Down, Home and End move further, and Enter,
Backspace or `q` close it. Dinah keeps the first 10000 lines a command writes
and says how many it left out.

Some commands cannot run inside the screen, and Dinah refuses them with
`dinah.not-in-tui` and says why. `mcp`, `lsp`, `serve`, `ui`, `completion` and
`tui` itself are refused. So are `view --watch` and `changes --wait`, because
the screen already redraws and waits for changes, and a value of `-`, because
the screen's keyboard is its standard input. Dinah does not detect every
command that waits for something: a command that never returns holds the
screen until it does, so run such a command from a shell.

Every command you type runs on the workbench, as the actor and in the
language the screen started with, and with the glyphs it started with. You
cannot give `--workbench`, `--actor` or `--lang` at the command line. If you
change any of those settings with `config set` there, Dinah writes it for the
next time `dinah tui` starts and tells you the screen keeps the value it
started with, and `dinah config` typed at the command line reports the values
the screen is using. `dinah config` at a shell reports what the file now
holds.

### Your own keys

You can bind a letter or a digit to a command line of your own, in your
settings, and press it in the lane and in a card read in full. For example,
this binds `o` to filing an open question on the selected card and labels it
in the footer:

    dinah config set tui.key.o "file $card open_question"
    dinah config set tui.label.o question

A binding may name three placeholders, which Dinah replaces when you press
the key. `$card` is the selected card's reference, `$column` is its column,
or the column of the lane you are in, written as the column's slug, and
`$view` is the view the screen draws. Each value becomes part of the one word
it stands in, so `list $card/checklist` lists the selected card's items, and
a value can never split into two words. A binding takes no arguments of its
own, so `$1` is refused, and a binding may not take a key the screen reads
itself, such as `a`, `t` or `x`. `dinah config set tui.key.<key>` with no
value removes the binding.

The footer lists each binding after the read keys, with its label, or with
its command line where you gave no label. It leaves out a binding whose
placeholder has nothing to stand for, such as `$card` in an empty lane, and
pressing its key says what it needs. It also leaves out a binding whose
command is a single act on the selected card that the workbench would refuse
you. Every other binding is always listed, and if the workbench refuses its
command, pressing the key shows the refusal. When the screen starts, it names
every binding in your settings that it will not run, and why.

### Pasting

If your terminal marks pastes, which most terminals do when a program asks,
a pasted line break never carries out a prompt, and Dinah waits for your own
Enter. The command line runs one line at a time, so it takes a paste of one
line, dropping a line break at its very end, and refuses a paste of several
lines, leaving what you had typed as it was. The filter prompt joins the
pasted lines into one. A paste into the comment prompt, or into any prompt
that holds several lines, keeps its line breaks. A paste made anywhere else is
ignored.

If your terminal does not mark pastes, Dinah cannot tell a paste from typing.
After Enter in the command line or the filter prompt it discards the keys that
were already waiting, but a long paste can arrive in several pieces, and a
piece that arrives after that is read as keys. A paste of more than one line
into the command line runs its first line and delivers the rest as keys, and
those keys can act on the selected card, because `a`, `b`, `t`, `r`, `m`, `c`
and `q` all act on a card or quit. So can any paste made while you are
looking at a lane. Keys you type ahead after Enter in those prompts are also
discarded. On such a terminal, paste only into the comment prompt, or paste
one line at a time.

### On Windows

On Windows the interface writes to the console only what Microsoft's "Console
Virtual Terminal Sequences" page lists, with two exceptions. The first is the
request that the console mark pastes, which the page does not list; where
your console ignores it, what the section above says about a terminal that
does not mark pastes applies. The second is narrower. Dinah hands the console
each screen in pieces, cut only between whole sequences and whole characters,
and reads back how many characters the console says it wrote. Microsoft does
not say when a console can write fewer characters than it was given, and if
one ever does in the middle of a sequence, Dinah writes the rest and draws
the whole screen again on the next frame, so a frame drawn wrongly lasts one
frame. Ctrl+L draws the whole screen again whenever you want it to.

### Handing the terminal to your editor

When you run `edit` from the screen, whether you type it at the command line,
choose it from the actions menu or press a key bound to it, Dinah ends the
screen, hands the terminal to your editor, and starts the screen again when
the editor ends. Dinah discards the keys you type while the editor starts or
closes, measures the window again, and draws the view at the window's new
size if you resized it meanwhile.

### How the interface ends

When you quit with `q` or Ctrl+C, Dinah leaves the alternate screen, shows
the cursor, switches off the paste marking it asked for, and puts your
terminal's settings back as it found them, and exits 0. Ctrl+Break on
Windows, or `kill -INT` on Linux and macOS, ends it the same way with exit
130. A closed console window or `kill` ends it the same way with exit 0.
Nothing can restore the terminal after `kill -KILL`, `taskkill /F` or a
hang-up.

If the interface stops on an error in Dinah itself, Dinah restores the
terminal first and then writes the error and where it happened. If you set
`TEA_DEBUG`, an error inside the library the interface is built on also
writes a log file into the current directory.
