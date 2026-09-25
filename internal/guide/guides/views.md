# Asking the same questions every day

A view is a question you ask your workbench often enough to give it a name.
You write it once as a set of queries, and `dinah view <name>` asks all of
them again and draws each answer as a section of its own.

    dinah view
    dinah view mine
    dinah view board
    dinah view waiting-on-me

`dinah view` with no name lists every view you can see. Dinah ships two views.
`dinah view mine` shows the cards you hold and the cards you blocked, and
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
  their column stands in the flow, and by arrival within one column.
- `collapsed` lists the columns a `columns` view draws as a count on one line
  rather than as a column, each named by reference. A `columns` view without
  it collapses the intake and done columns, and `collapsed: []` collapses
  nothing. It has no effect on the `list` layout, and Dinah ignores an entry
  naming a column this workbench does not have.
- `sections` lists the view's questions in the order they are drawn. Each
  section carries a `query`, written in the language the guide on queries
  teaches, and may carry a `title`. A section without a title is headed by its
  query, exactly as you wrote it.

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
3. the views Dinah ships, which today are `mine` and `board`.

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

To change either view, declare your own view of the same name, in your
settings or on the workbench, and yours is drawn in its place. To draw the
board with a shorter command, give yourself an alias named board:

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
