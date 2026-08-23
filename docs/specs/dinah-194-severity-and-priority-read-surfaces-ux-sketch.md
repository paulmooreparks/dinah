# dinah-194 UX sketch: a reader can see a card's severity and priority

Every block below is headed **Proposed** and is drawn by hand against this card's spec. None
of it is quoted from a running binary, because the fields do not exist on any read surface
yet and the spec stage does not build. Column widths follow the shapes `dinah ls` and
`dinah show` already print, taken from the shipped `render.go` and the catalog headings it
already carries (`Card`, `Standing`, `Title`), so the drawing is consistent with the product
rather than measured against it. Treat a width here as intent and not as a measurement.

The scratch workbench these transcripts run in declares both level sets, the same board
dinah-193's sketch used:

```yaml
levels:
  severity: [trivial, minor, major, critical]
  priority: [later, soon, next, now]
```

## 1. `dinah ls`, today

What the command prints now, unchanged by this card unless a card in the state carries a
level:

```text
$ dinah ls acceptance
  Card       Standing  Title
  ---------  --------  --------------------------------------------
  dinah-197  ready     A workbench states its default lane
  dinah-199  active    Rename reaches the archived half too
```

## 2. `dinah ls`, with severity and priority populated

Proposed. Two new columns, Severity and Priority, sit between Standing and Title. Both
axes' order follows `LevelAxes`: severity before priority.

```text
$ dinah ls acceptance
  Card       Standing  Severity  Priority  Title
  ---------  --------  --------  --------  --------------------------------
  dinah-197  ready     major     now       A workbench states its default lane
  dinah-199  active                        Rename reaches the archived half too
  dinah-200  ready     minor               A long title breaks the table it sits in
```

`dinah-199` carries neither field: both cells print blank, the same way an empty cell
prints anywhere else in this table today. `dinah-200` carries a severity and no priority:
the Priority cell is blank on that one row while the column still draws, because a
different row in the same listing populates it.

## 3. `dinah ls`, on a listing where nobody has classified anything

Proposed. When no card in the listing carries a value for an axis, that axis's column is
dropped entirely rather than drawn empty. This is the table layout's own existing rule
(`withoutEmptyColumns`, cmd/dinah/table.go): a column no row fills is not printed, heading
and all. Nothing new is written to produce this; the two new columns fall under a rule that
already governs every column in the table.

```text
$ dinah ls acceptance
  Card       Standing  Title
  ---------  --------  --------------------------------------------
  dinah-201  ready     Fresh cards, none classified yet
```

The same rule means a workbench that declares only one axis, or neither, never grows a
column its cards cannot populate: the column's presence tracks what the visible cards
actually carry, not what the workbench declares.

## 4. `dinah show`, today

```text
$ dinah show dinah-197
dinah-197  A workbench states its default lane  [acceptance / ready]
```

## 5. `dinah show`, with severity and priority set

Proposed. Two new lines, printed only when the field is non-empty, directly under the
card's own summary line and ahead of the holder line and the blocked line.

```text
$ dinah show dinah-197
dinah-197  A workbench states its default lane  [acceptance / ready]
  severity: major
  priority: now
```

Proposed. A card carrying one axis and not the other prints one line and not the other.

```text
$ dinah show dinah-200
dinah-200  A long title breaks the table it sits in  [acceptance / ready]
  severity: minor
```

Proposed. A card carrying neither axis prints neither line, unchanged from section 4.

Proposed. The two new lines sit ahead of the holder and blocked lines, since severity and
priority are attributes filed with the card, not runtime claim state. A held, blocked card
carrying both fields prints all four lines in this order:

```text
$ dinah show dinah-199
dinah-199  Rename reaches the archived half too  [acceptance / active]
  severity: critical
  priority: soon
  held by paul
  blocked: waiting on a decision from dinah-190
```

## 6. The machine form

Proposed. `--json` on `show`, on `ls`, and on every act that already returns a card (claim,
move, release, block, unblock, add, join, leave — the whole `CardView` contract) gains
`severity` and `priority`, each omitted from the object when the card carries no value for
that axis, rather than emitted as an empty string.

```text
$ dinah show dinah-197 --json
{
  "outcome": "ok",
  "verb": "show",
  "card": {
    "id": "a1b2c3d4e5f6",
    "ref": "dinah-197",
    "title": "A workbench states its default lane",
    "state": "acceptance",
    "state_title": "Acceptance",
    "substate": "ready",
    "severity": "major",
    "priority": "now",
    "revision": "9f2c1a04bb31"
  },
  "affordances": ["move", "claim"]
}
```

Proposed. A card carrying neither field carries neither key:

```text
$ dinah show dinah-201 --json
{
  "outcome": "ok",
  "verb": "show",
  "card": {
    "id": "7e6d5c4b3a29",
    "ref": "dinah-201",
    "title": "Fresh cards, none classified yet",
    "state": "acceptance",
    "state_title": "Acceptance",
    "substate": "ready",
    "revision": "22b8f0a1c9de"
  },
  "affordances": ["move", "claim"]
}
```

## 7. A level the workbench no longer declares

Proposed. dinah-193's D-2 fixed the posture: a read validates nothing. A card whose stored
severity or priority names a level this workbench's declaration for that axis does not
carry is shown exactly as it stands, on all three surfaces, with no marker that it is
unrecognized. `dinah check` is the only place the mismatch is reported.

```text
$ dinah ls acceptance
  Card       Standing  Severity  Title
  ---------  --------  --------  --------------------------------------------
  dinah-202  ready     urgent    A card whose severity nobody declares any more

$ dinah show dinah-202
dinah-202  A card whose severity nobody declares any more  [acceptance / ready]
  severity: urgent

$ dinah show dinah-202 --json
{
  "outcome": "ok",
  "verb": "show",
  "card": {
    "id": "5f4e3d2c1b0a",
    "ref": "dinah-202",
    "title": "A card whose severity nobody declares any more",
    "state": "acceptance",
    "state_title": "Acceptance",
    "substate": "ready",
    "severity": "urgent",
    "revision": "c4a1908fe23b"
  },
  "affordances": ["move", "claim"]
}
```

## 8. What stays as it is

`dinah query`'s table (`renderMatches`) is not touched here; it is dinah-195's surface, and
the query language cannot yet name either field. `dinah tree` is not touched. `dinah
status`'s Holding and Blocked tables print only a card's reference and title (or its block
reason) today and continue to; they are not one of the three surfaces this card owns. No
hint or rank is looked up or shown anywhere: only the level name the card stores is
displayed, on every surface, whether or not the workbench still declares it.
