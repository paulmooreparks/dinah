# dinah-151 UX sketch: the two tree verbs

Every block below came out of a built binary. The tree in each one is the
`dinah-151` contract added to a scratch clone at commit `82f0d38`, built, and run
against a workbench this sketch created. Nothing here is drawn by hand. The
prototype was a measuring instrument and was thrown away; the implementer writes
the shipped code from the spec.

The fixture holds six cards. Dinah's operator holds `tw-1` in Doing, `alka` holds
`tw-2` in Doing, `tw-3` is blocked in Intake, and three cards sit ready in Intake.
Four of the six belong to one or both of two workstreams. `tw-1` carries two
comments, two checklist items, and one attachment.

## 1. `dinah tree`, the no-argument preset

You get states over substates over cards, which is the status tree.

```text
  Node              What      Count
  ----------------  --------  -----
  tw                wb2       6
  |-- intake        Intake    4
  |   |-- ready     substate  3
  |   |   |-- tw-4  Draw the guides
  |   |   |-- tw-5  Translate the headings
  |   |   `-- tw-6  Retire the second map
  |   |-- active    substate  0
  |   `-- blocked   substate  1
  |       `-- tw-3  Count what is hidden
  |-- doing         Doing     2
  |   |-- ready     substate  0
  |   |-- active    substate  2
  |   |   |-- tw-1  Ship the tree verb
  |   |   `-- tw-2  Name the depth levels
  |   `-- blocked   substate  0
  `-- done          Done      0
      |-- ready     substate  0
      |-- active    substate  0
      `-- blocked   substate  0
```

The Count column is blank on a card. A card is one card, and the number would say
nothing that the row does not already say. Every group carries the number of cards at or below it,
including the groups that carry none. Both `state` and `substate` are closed sets,
so Dinah draws a group for every member of each and an empty group is visible
rather than absent.

## 2. `dinah tree --depth groups`, and what a truncated node reports

```text
  Node             What      Count  Not shown
  ---------------  --------  -----  -----------
  tw               wb2       6
  |-- intake       Intake    4
  |   |-- ready    substate  3      3 not shown
  |   |-- active   substate  0
  |   `-- blocked  substate  1      1 not shown
  |-- doing        Doing     2
  |   |-- ready    substate  0
  |   |-- active   substate  2      2 not shown
  |   `-- blocked  substate  0
  `-- done         Done      0
      |-- ready    substate  0
      |-- active   substate  0
      `-- blocked  substate  0
```

Read the Count column against the Not shown column. The tree stopped at the
groups, so no card is drawn anywhere, and every group whose count is not zero
says how many cards it is holding back. A group of zero cards hides nothing and
says nothing.

## 3. `dinah tree --group-by workstream,state`, where the counts stop adding up

A card belongs to any number of workstreams, so grouping on that axis puts one
card under several parents. This is the block to read closely.

```text
  Node              What        Count
  ----------------  ----------  -----
  tw                wb2         6
  |-- 0f1e2d3c4b5a  workstream  3
  |   |-- intake    Intake      2
  |   |   |-- tw-4  Draw the guides
  |   |   `-- tw-5  Translate the headings
  |   |-- doing     Doing       1
  |   |   `-- tw-1  Ship the tree verb
  |   `-- done      Done        0
  |-- a1b2c3d4e5f6  workstream  3
  |   |-- intake    Intake      1
  |   |   `-- tw-4  Draw the guides
  |   |-- doing     Doing       2
  |   |   |-- tw-1  Ship the tree verb
  |   |   `-- tw-2  Name the depth levels
  |   `-- done      Done        0
  `-- (no value)    workstream  2
      |-- intake    Intake      2
      |   |-- tw-3  Count what is hidden
      |   `-- tw-6  Retire the second map
      |-- doing     Doing       0
      `-- done      Done        0
```

The three top-level groups count 3, 3, and 2, which is 8, and the root counts 6.
Both numbers are right. `tw-1` and `tw-4` each belong to two workstreams and are
drawn under both, so the sum over the children counts them twice, while the root
counts each card once. An implementation that derived a node's count by adding up
its children would print 8 at the root and look entirely plausible, so this case is
one of the acceptance criteria.

The `(no value)` group holds the cards in no workstream. Dinah draws that group
whenever the axis leaves any card unset.

## 4. `dinah tree --group-by holder`, an open-valued axis

```text
  Node            What                    Count
  --------------  ----------------------  -----
  tw              wb2                     6
  |-- alka        holder                  1
  |   `-- tw-2    Name the depth levels
  |-- paul        holder                  1
  |   `-- tw-1    Ship the tree verb
  `-- (no value)  holder                  4
      |-- tw-3    Count what is hidden
      |-- tw-4    Draw the guides
      |-- tw-5    Translate the headings
      `-- tw-6    Retire the second map
```

Nobody declares the set of holders in advance. Dinah therefore draws a group only
for a holder that some card actually names, which means a person who has never
held a card here never appears. Compare that with block 1, where the flow
declares every state and every state gets a group whether or not it holds a card.

## 5. `dinah tree "substate:ready"`, and what the filter hides

```text
  Node              What      Count  Not shown
  ----------------  --------  -----  --------------
  tw                wb2       3      3 filtered out
  |-- intake        Intake    3      1 filtered out
  |   |-- ready     substate  3
  |   |   |-- tw-4  Draw the guides
  |   |   |-- tw-5  Translate the headings
  |   |   `-- tw-6  Retire the second map
  |   |-- active    substate  0
  |   `-- blocked   substate  0      1 filtered out
  |-- doing         Doing     0      2 filtered out
  |   |-- ready     substate  0
  |   |-- active    substate  0      2 filtered out
  |   `-- blocked   substate  0
  `-- done          Done      0
      |-- ready     substate  0
      |-- active    substate  0
      `-- blocked   substate  0
```

Every node answers both questions at once. The Count column says what survived the
filter at or below that node, and the Not shown column says how many the filter
removed from the same place. Doing holds two cards and the filter took both, so
the group is drawn with a count of zero rather than dropped, and you can see that
the work is there rather than absent.

Depth and filter can hide from the same node, and they are reported separately.
Adding `--depth groups` to the same query gives this.

```text
  Node             What      Count  Not shown
  ---------------  --------  -----  --------------
  tw               wb2       3      3 filtered out
  |-- intake       Intake    3      1 filtered out
  |   |-- ready    substate  3      3 not shown
  |   |-- active   substate  0
  |   `-- blocked  substate  0      1 filtered out
  |-- doing        Doing     0      2 filtered out
  |   |-- ready    substate  0
  |   |-- active   substate  0      2 filtered out
  |   `-- blocked  substate  0
  `-- done         Done      0
      |-- ready    substate  0
      |-- active   substate  0
      `-- blocked  substate  0
```

## 6. `dinah contents tw-1`, the containment walk

```text
  Node               What                        Count
  -----------------  --------------------------  -----
  tw-1               Ship the tree verb          5
  |-- comments/1     the first note              0
  |-- comments/2     the second note             0
  |-- checklist/1    the node form is one shape  0
  |-- checklist/2    which verb names the walk   0
  `-- attachments/1  sample.txt                  0
```

Every label in the Node column is a reference `dinah show`, `dinah path`, and
`dinah edit` already accept, so you can copy one out of the tree and open it. The
walk reads the same containment table for a declared extension kind as it does for
a comment, so an extension kind appears here without any code that knows its name.

## 7. `dinah contents --depth cards`, from the workbench

```text
  Node        What                    Count  Not shown
  ----------  ----------------------  -----  -----------
  tw          wb2                     14
  |-- intake  Intake                  0
  |-- doing   Doing                   0
  |-- done    Done                    0
  |-- tw-1    Ship the tree verb      5      5 not shown
  |-- tw-3    Count what is hidden    0
  |-- tw-4    Draw the guides         0
  |-- tw-5    Translate the headings  0
  |-- tw-6    Retire the second map   0
  `-- tw-2    Name the depth levels   0
```

The states come in the flow's declared order and the cards come in the profile's
queue order, which is the order `dinah ls` already prints. The root counts 14
entities, which is three states, six cards, and the five entities under `tw-1`.

## 8. A root that contains nothing

```text
$ dinah contents tw-1/comments/1
tw-1/comments/1 contains nothing.
```

A leaf root is an ordinary answer rather than a refusal. Dinah says so in one
sentence.

## 9. The refusals, in English and in Hindi

```text
$ dinah tree --group-by priority
dinah.unknown-axis priority is not a group-by axis; the axes are block_kind, holder, state, substate, and workstream
$ echo $?
2
```

```text
$ dinah --lang hi tree --group-by priority
dinah.unknown-axis priority समूहन अक्ष नहीं है; अक्ष हैं block_kind, holder, state, substate, और workstream
```

The axis names stay in Latin script under every language. They are what you type.

## 10. The two lines the help block gains

```text
  guide [topic]                          the embedded guides, or one of them
  tree [query] [--group-by <axes>] [--depth <level>]
                                         the workbench as a tree, grouped
  contents <ref> [--depth <level>]       what an entity contains, all the way down
```

The query is `tree`'s free-text slot, which is character-for-character the string
`dinah query` takes, so you write one filter and it means the same thing in both
commands.

## 11. A window too narrow for the tree, which this card does not fix

At 34 columns the table neither holds its columns nor falls back to the stacked
form, and the tree stops being readable.

```text
  Node   What   Count  Not shown
  -----  -----  -----  -----------
  tw     wb2    6
  |-- intake
         Intake 4
  |   |-- ready
         substate
              3      3 not shown
  |   |-- active
         substate
              0
```

The cause is in the shared table machinery rather than in the tree. The stacked
fallback fires only when a column stands at its own heading and a field overruns
it, and the Node column here measures wider than the heading `Node`, so the
fallback never fires and every row overflows instead. `dinah-132` owns that
machinery and is in Acceptance, so this card names the case and spawns a card for
it rather than editing the predicate underneath another card in flight.
