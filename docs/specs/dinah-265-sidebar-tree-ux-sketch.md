# dinah-265 UX sketch: the sidebar tree

VS Code renders a `TreeItem` with its own icon, label, description and
tooltip; there is no table and no ASCII art in the product itself. This sketch
draws what those fields hold, row by row, in the same fixture dinah-151 used
(workbench `tr`, "Trees", six cards: `tr-1` held by the operator and `tr-2`
held by `alka` in Doing, `tr-3` blocked in Intake, `tr-4`/`tr-5`/`tr-6` ready
in Intake), plus one row from a second fixture that declares
`awaiting_outside` and one that declares a WIP limit, since `tr` has neither.

Each row below is written `[icon] Label — description` with the tooltip on
the following indented line, which is the same shape VS Code's own
`vscode.TreeItem` documentation uses in its examples: `label` is what prints,
`description` is the dim trailing text, `tooltip` is the hover text, and
`iconPath`/`ThemeIcon` is bracketed. A `>` prefix marks a collapsible row.

## 1. The ordinary case: `tr` open, `Intake` expanded

```
> [$(book)] Trees
> [ ] Intake — 4, taken
      Doing (0/0)... (collapsed, not drawn)
      Blocked... (collapsed, not drawn)
  > [ ] Ready — 3
        [$(circle-outline)] Draw the guides
            tr-4 · Intake · ready
        [$(circle-outline)] Translate the headings
            tr-5 · Intake · ready
        [$(circle-outline)] Retire the second map
            tr-6 · Intake · ready
    [ ] Active — 0 (no children; VS Code draws nothing under a childless row)
  > [ ] Blocked — 1
        [$(circle-slash), red] tr-3
            tr-3 · Intake · blocked · workaround, filed by alka
```

The workbench row's label is the resolved title (`Trees`), matching the
status bar. The column row's description is the Work-column word the CLI's
`columns.work.*` catalog already prints (`taken` here, since Intake is an
ordinary work column cards are claimed from) followed by the count, or the
count against a limit where one is declared (section 3). The description
carries the word first, ahead of the count, because the word is the thing
OQ-2 exists to teach: a ready card under this column offers a Claim item on
its own row, and this row is what a reader glances at first to learn
whether that is even worth expecting.

A state group with zero children (`Active`) is drawn with `count == 0`
and no expand arrow, because `getChildren` on it returns `[]` and VS Code
never draws an arrow on a childless row. It is drawn rather than omitted,
matching dinah-151's own closed-enum rule for the state axis: absence
would read as "no such thing as Active here" rather than "nothing is
standing there right now."

## 2. A card's icon by state

```
ready:    $(circle-outline)                    (no color; default foreground)
active:   $(circle-filled)   charts.blue
blocked:  $(circle-slash)    charts.red
```

A card's own state (ready, active, blocked) is never printed as text on
its row; the icon and its `Doing` / `Blocked` group ancestor already say
it, and repeating it in the label would be the same redundancy dinah-151
avoids by leaving a group node's own title empty.

## 3. A capacity-limited column

A column declaring a WIP limit shows the count against it, in the same
order `renderColumns` already composes (`count + "/" + capacity`):

```
[ ] Doing — 2/3, taken
```

## 4. `awaiting_outside`

A second fixture, a column called "Customer approval" carrying
`awaiting_outside: true` and holding two ready cards:

```
> [ ] Customer approval — 2, waiting
      [$(circle-outline)] Confirm the pricing page
          cp-7 · Customer approval · ready
      [$(circle-outline)] Confirm the launch date
          cp-8 · Customer approval · ready
```

No `Ready` group heading sits above the two cards. Revised per the
operator's own correction (OQ-4, 2026-08-27): "There isn't a Ready state in
such a column, because work is not claimed in that column. It's a variation
on a pull-queue column." `awaiting_outside: true` makes the column's own
`TakesWorkUp` false, the same as an intake, done, or buffer column, and per
OQ-2's three-tier rule a column that takes no work up draws no
ready-versus-active breakdown at all, so the cards render directly beneath
the column row instead, exactly as an occupied intake or buffer column does
once dinah-322 lands (this card's own gate). The earlier drawing of this
section, a `Ready — 2` group between the column row and the two cards, is
withdrawn.

The description reads `waiting` in place of `taken`, sourced from the same
`columns.work.waiting` word the CLI prints. Right-clicking `cp-7` still opens
Move and Block only; there is no Claim item at all, not a greyed-out one, an
absent one, which is what section 5's context-menu table means by "the
column row is where the explanation lives, not a disabled item on the
card." A reader who right-clicks a card under a `waiting` column and finds
Move and Block alone has already been told why on the row one level up.

## 5. The context menu, by contextValue

Four `contextValue` strings, four menus. `Open` is every row's default click
and is not a context-menu entry.

| contextValue | Row it applies to | Menu items |
| --- | --- | --- |
| `dinah.card.ready.claim` | ready, own column takes work up, not `awaiting_outside` | Claim, Move, Block |
| `dinah.card.ready.none` | ready, own column does not take work up (buffer, or `awaiting_outside`), or take-up act undetermined | Move, Block |
| `dinah.card.active` | active | Release, Move, Block |
| `dinah.card.blocked` | blocked | Unblock |

`tr-4`, `tr-5`, `tr-6` above are all `dinah.card.ready.claim`, since Intake
takes work up and is not `awaiting_outside`. `cp-7` and `cp-8` are
`dinah.card.ready.none`. `tr-1` and `tr-2` (Doing, active) are
`dinah.card.active`. `tr-3` is `dinah.card.blocked`.

## 6. Move

Move has no preview in this sketch, because its destination list is fetched
on demand (`dinah --json instructions <ref>`) when the item is invoked
rather than eagerly for every card in the tree. A quick-pick opens naming
each `LegalMove.Title`, `forward` entries first in the array's own order,
then `backward` ones; picking one runs `dinah move <ref> <column>`, `<column>`
being the picked entry's own `Column` field (dinah-287 renamed this field
from `State`).

## 7. A refused-resolution row in a multi-root window

Two folders, one resolving cleanly and one ambiguous:

```
> [$(book)] Trees
    ... (as above)
[$(warning)] second-folder: several workbenches reachable
    Set dinah.workbench for this folder to choose one.
```

The second line is not collapsible, carries no children and no
contextValue, and its label names the folder VS Code shows in its own
multi-root UI (`folder.name`) so the row is legible without opening the
tooltip. Wording matches the ambiguous `viewsWelcome` block verbatim, so a
user who has seen that text once recognises it here.

## 8. A workspace folder holding no workbench of its own, several nested beneath it

Added 2026-08-27, after dinah-281 landed the downward walk this section
draws on. The operator's own layout: one workspace folder, `customers`,
holding two customer directories, each with its own `.dinah` two levels
down, and the folder itself has none.

```
> [$(book)] Acme Co
    ... (that workbench's own subtree, exactly as section 1 draws it)
> [$(book)] Bell Industries
    ... (that workbench's own subtree, exactly as section 1 draws it)
```

Both rows come from one `dinah --json tree --root <customers-folder>` call
(joined with `status --root` and `ls --root` the same way a single
workbench is joined today), not from two spawns. Each root row is a
`workbenchForest` row rather than a `workbenchRoot` row, carrying the
resolved workbench's title and slug and, in its `description`, the walked
path relative to `customers` (`acme/board`, `bell/tracker`) so two
same-titled customers are still told apart. Expanding one is free: the
subtree already arrived with the walk, there is nothing further to fetch.

## 9. Three ways a root row can fail to answer, drawn differently

A downward walk can meet a directory it cannot read, a workbench whose
anchor will not open, and a workbench that opened and declined the one
question this checkpoint asked it. dinah-281 puts these on two separate
wire fields (a walked row's own `refused`, and the per-read `unanswered`)
precisely so a reader draws three different rows rather than one generic
failure.

```
[$(warning)] acme/scratch — unreadable
    The walk could not read this directory.
> [$(book)] Bell Industries — would not open
    [$(warning)] This workbench's definition would not open (dinah.unreadable-workbench).
> [$(book)] Carter LLP
    ... (ordinary subtree)
```

`acme/scratch` carries a `refused` name and no title or slug at all (the
walk never got far enough to read one), so its row is drawn from its path
alone, not collapsible, no context menu: the same treatment section 7 gives
the narrow ambiguous-folder dead end. `Bell Industries` carries a `refused`
name too, but its anchor gave up a title before failing to open, so the row
keeps that identity and is told, directly beneath it, that the workbench
would not open; it is still collapsed by default since there is nothing
under it to show. `Carter LLP` carries neither `refused` nor `unanswered`
and draws its ordinary subtree exactly as section 8's rows do.

A row carrying `unanswered` rather than `refused` (this checkpoint's own
`tree` call refused a query the workbench itself would otherwise have
answered, an ordinary race rather than the common case) keeps its identity
and its already-known subtree from the last good checkpoint, and is told
underneath, in the same place `unopened` is told, that this refresh
specifically did not answer.
