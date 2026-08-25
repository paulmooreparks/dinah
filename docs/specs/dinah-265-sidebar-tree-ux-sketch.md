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
status bar. The state row's description is the Work-column word the CLI's
`states.work.*` catalog already prints (`taken` here, since Intake is an
ordinary work state cards are claimed from) followed by the count, or the
count against a limit where one is declared (section 3). The description
carries the word first, ahead of the count, because the word is the thing
Q2 exists to teach: a ready card under this state offers a Claim item on its
own row, and this row is what a reader glances at first to learn whether
that is even worth expecting.

A substate group with zero children (`Active`) is drawn with `count == 0`
and no expand arrow, because `getChildren` on it returns `[]` and VS Code
never draws an arrow on a childless row. It is drawn rather than omitted,
matching dinah-151's own closed-enum rule for `substate`: absence would read
as "no such thing as Active here" rather than "nothing is standing there
right now."

## 2. A card's icon by substate

```
ready:    $(circle-outline)                    (no color; default foreground)
active:   $(circle-filled)   charts.blue
blocked:  $(circle-slash)    charts.red
```

Substate is never printed as text on a card row; the icon and its `Doing` /
`Blocked` group ancestor already say it, and repeating it in the label would
be the same redundancy dinah-151 avoids by leaving a group node's own title
empty.

## 3. A capacity-limited state

A state declaring a WIP limit shows the count against it, in the same order
`renderStates` already composes (`count + "/" + capacity`):

```
[ ] Doing — 2/3, taken
```

## 4. `awaiting_outside`

A second fixture, a state called "Customer approval" carrying
`awaiting_outside: true` and holding two ready cards:

```
> [ ] Customer approval — 2, waiting
  > [ ] Ready — 2
        [$(circle-outline)] Confirm the pricing page
            cp-7 · Customer approval · ready
        [$(circle-outline)] Confirm the launch date
            cp-8 · Customer approval · ready
```

The description reads `waiting` in place of `taken`, sourced from the same
`states.work.waiting` word the CLI prints. Right-clicking `cp-7` opens Move
and Block. There is no Claim item at all — not a greyed-out one, an absent
one — which is what section 5's context-menu table means by "the state row
is where the explanation lives, not a disabled item on the card." A reader
who right-clicks a card under a `waiting` state and finds Move and Block
alone has already been told why on the row one level up.

## 5. The context menu, by contextValue

Four `contextValue` strings, four menus. `Open` is every row's default click
and is not a context-menu entry.

| contextValue | Row it applies to | Menu items |
| --- | --- | --- |
| `dinah.card.ready.claim` | ready, own state takes work up, not `awaiting_outside` | Claim, Move, Block |
| `dinah.card.ready.none` | ready, own state does not take work up (buffer, or `awaiting_outside`), or take-up act undetermined | Move, Block |
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
then `backward` ones; picking one runs `dinah move <ref> <state>`.

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
