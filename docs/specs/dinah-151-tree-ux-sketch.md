# dinah-151 UX sketch: the two tree verbs

Every table below was laid out by the tool's own table renderer, driven with the
row data a prototype produced by running against a workbench it created. The
prototype was a
measuring instrument and was thrown away; the implementer writes the shipped
code from the spec. The sentence printed above each table is new in this cycle
and is drawn here as the spec says it reads.

The fixture is a workbench whose slug is `tr` and whose title is `Trees`. It
holds six cards. Dinah's operator holds `tr-1` in Doing, `alka` holds `tr-2` in
Doing, `tr-3` is blocked in Intake, and three cards sit ready in Intake. Four of
the six belong to one or both of two workstreams. `tr-1` carries two comments,
two checklist items, and one attachment.

The workbench is not a row. Its slug, its title, and its total print on the line
above the tree, because they are a given for the whole command, and the tree
itself starts at the states. `dinah contents` does the same with the entity you
named.

Three columns follow the reference on every row. Entity says what sort of thing
the row is, Title says what it is called, and Count says how many subjects sit
under it. A group node is not an entity and has no title of its own, so its
Entity cell carries the axis it was grouped on and its Title cell is empty. That
is also what tells you a cell like `ready` is not an address.

## 1. `dinah tree`, the no-argument preset

You get states over substates over cards, which is the status tree.

```text
Trees (tr) holds 6 cards.

  Reference     Entity    Title   Count
  ------------  --------  ------  -----
  intake        state     Intake  4
  |-- ready     substate          3
  |   |-- tr-4  card      Draw the guides
  |   |-- tr-5  card      Translate the headings
  |   `-- tr-6  card      Retire the second map
  |-- active    substate          0
  `-- blocked   substate          1
      `-- tr-3  card      Count what is hidden
  doing         state     Doing   2
  |-- ready     substate          0
  |-- active    substate          2
  |   |-- tr-1  card      Ship the tree verb
  |   `-- tr-2  card      Name the depth levels
  `-- blocked   substate          0
  done          state     Done    0
  |-- ready     substate          0
  |-- active    substate          0
  `-- blocked   substate          0
```

The Count column is blank on a card. A card is one card, and the number would say
nothing that the row does not already say. Every group carries the number of cards at or below it,
including the groups that carry none. Both `state` and `substate` are closed sets,
so Dinah draws a group for every member of each and an empty group is visible
rather than absent. No row carries a Not shown cell, so that column is not drawn
at all.

## 2. `dinah tree --depth groups`, and what a truncated node reports

```text
Trees (tr) holds 6 cards.

  Reference    Entity    Title   Count  Not shown
  -----------  --------  ------  -----  -----------
  intake       state     Intake  4
  |-- ready    substate          3      3 not shown
  |-- active   substate          0
  `-- blocked  substate          1      1 not shown
  doing        state     Doing   2
  |-- ready    substate          0
  |-- active   substate          2      2 not shown
  `-- blocked  substate          0
  done         state     Done    0
  |-- ready    substate          0
  |-- active   substate          0
  `-- blocked  substate          0
```

A node reports what its own children were cut off, and nothing more. The substate
groups are the deepest rows this depth admits, so each one holding a card says how
many it is holding back, and the state above it says nothing at all, because a
state at this depth has every child it owns drawn beneath it and hides none of
them itself. Read the two columns together. `intake` still counts 4, and the three
rows under it account for all four.

## 3. `dinah tree --group-by workstream,state`, where the counts stop adding up

A card belongs to any number of workstreams, so grouping on that axis puts one
card under several parents. This is the block to read closely.

```text
Trees (tr) holds 6 cards.

  Reference     Entity      Title   Count
  ------------  ----------  ------  -----
  0f1e2d3c4b5a  workstream          3
  |-- intake    state       Intake  2
  |   |-- tr-4  card        Draw the guides
  |   `-- tr-5  card        Translate the headings
  |-- doing     state       Doing   1
  |   `-- tr-1  card        Ship the tree verb
  `-- done      state       Done    0
  a1b2c3d4e5f6  workstream          3
  |-- intake    state       Intake  1
  |   `-- tr-4  card        Draw the guides
  |-- doing     state       Doing   2
  |   |-- tr-1  card        Ship the tree verb
  |   `-- tr-2  card        Name the depth levels
  `-- done      state       Done    0
  (no value)    workstream          2
  |-- intake    state       Intake  2
  |   |-- tr-3  card        Count what is hidden
  |   `-- tr-6  card        Retire the second map
  |-- doing     state       Doing   0
  `-- done      state       Done    0
```

The three top-level groups count 3, 3, and 2, which is 8, and the line above the
tree counts 6. Both numbers are right. `tr-1` and `tr-4` each belong to two
workstreams and are drawn under both, so the sum over the groups counts them
twice, while the workbench counts each card once. An implementation that derived
a node's count by adding up its children would report 8 for the workbench and
look entirely plausible, so this case is one of the acceptance criteria.

The `(no value)` group holds the cards in no workstream. Dinah draws that group
whenever the axis leaves any card unset. A workstream group's Title cell is empty
because the tool cannot yet read a workstream's own title, and the group is
labelled with the identifier the card carries.

## 4. `dinah tree --group-by holder`, an open-valued axis

```text
Trees (tr) holds 6 cards.

  Reference   Entity  Title  Count
  ----------  ------  -----  -----
  alka        holder         1
  `-- tr-2    card    Name the depth levels
  paul        holder         1
  `-- tr-1    card    Ship the tree verb
  (no value)  holder         4
  |-- tr-3    card    Count what is hidden
  |-- tr-4    card    Draw the guides
  |-- tr-5    card    Translate the headings
  `-- tr-6    card    Retire the second map
```

Nobody declares the set of holders in advance. Dinah therefore draws a group only
for a holder that some card actually names, which means a person who has never
held a card here never appears. Compare that with block 1, where the flow
declares every state and every state gets a group whether or not it holds a card.

## 5. `dinah tree "substate:ready"`, and what the filter hides

```text
Trees (tr) holds 6 cards, of which 3 match the query.

  Reference     Entity    Title   Count  Not shown
  ------------  --------  ------  -----  --------------
  intake        state     Intake  3      1 filtered out
  |-- ready     substate          3
  |   |-- tr-4  card      Draw the guides
  |   |-- tr-5  card      Translate the headings
  |   `-- tr-6  card      Retire the second map
  |-- active    substate          0
  `-- blocked   substate          0      1 filtered out
  doing         state     Doing   0      2 filtered out
  |-- ready     substate          0
  |-- active    substate          0      2 filtered out
  `-- blocked   substate          0
  done          state     Done    0
  |-- ready     substate          0
  |-- active    substate          0
  `-- blocked   substate          0
```

Every node answers both questions at once. The Count column says what survived the
filter at or below that node, and the Not shown column says how many the filter
removed from the same place. Doing holds two cards and the filter took both, so
the group is drawn with a count of zero rather than dropped, and you can see that
the work is there rather than absent.

Depth and filter can hide from the same node, and they are reported separately.
Adding `--depth groups` to the same query gives this.

```text
Trees (tr) holds 6 cards, of which 3 match the query.

  Reference    Entity    Title   Count  Not shown
  -----------  --------  ------  -----  --------------
  intake       state     Intake  3      1 filtered out
  |-- ready    substate          3      3 not shown
  |-- active   substate          0
  `-- blocked  substate          0      1 filtered out
  doing        state     Doing   0      2 filtered out
  |-- ready    substate          0
  |-- active   substate          0      2 filtered out
  `-- blocked  substate          0
  done         state     Done    0
  |-- ready    substate          0
  |-- active   substate          0
  `-- blocked  substate          0
```

## 6. `dinah contents tr-1`, the containment walk

```text
Ship the tree verb (tr-1) contains 5 entities.

  Reference      Entity      Title                       Count
  -------------  ----------  --------------------------  -----
  comments/1     comment     the first note              0
  comments/2     comment     the second note             0
  checklist/1    item        the node form is one shape  0
  checklist/2    item        which verb names the walk   0
  attachments/1  attachment  sample.txt                  0
```

Every label in the Reference column is a reference `dinah show`, `dinah path`, and
`dinah edit` already accept once you put the card in front of it, so you can copy
one out of the tree and open it. The walk reads the same containment table for a
declared extension kind as it does for a comment, so an extension kind appears
here without any code that knows its name.

## 7. `dinah contents --depth cards`, from the workbench

```text
Trees (tr) contains 14 entities.

  Reference  Entity  Title                   Count  Not shown
  ---------  ------  ----------------------  -----  -----------
  intake     state   Intake                  0
  doing      state   Doing                   0
  done       state   Done                    0
  tr-1       card    Ship the tree verb      5      5 not shown
  tr-3       card    Count what is hidden    0
  tr-4       card    Draw the guides         0
  tr-5       card    Translate the headings  0
  tr-6       card    Retire the second map   0
  tr-2       card    Name the depth levels   0
```

The states come in the flow's declared order and the cards come in the profile's
queue order, which is the order `dinah ls` already prints. The workbench contains
14 entities, which is three states, six cards, and the five entities under `tr-1`.
The depth stopped at the cards, so `tr-1` says it is holding five back and every
other card says nothing, because every other card holds nothing.

## 8. A root that contains nothing

```text
$ dinah contents tr-1/comments/1
the first note (tr-1/comments/1) contains nothing.
```

A leaf root is an ordinary answer rather than a refusal. The line that would carry
the count carries the whole answer instead, and no table follows it.

## 9. The refusals, in English and in Hindi

```text
$ dinah tree --group-by priority
dinah.unknown-axis priority is not a group-by axis; the axes are actor, block_kind, entered, event, holder, left, state, substate, and workstream
$ echo $?
2
```

```text
$ dinah tree --group-by at
dinah.unknown-axis at is not a group-by axis; the axes are actor, block_kind, entered, event, holder, left, state, substate, and workstream
```

`priority` is a word the tool does not know at all. `at` is a field you may filter
on and cannot group on, because an instant is a different value on every act and
nothing has chosen the bucket a group would stand for.

```text
$ dinah --lang hi tree --group-by priority
dinah.unknown-axis priority समूहन अक्ष नहीं है; अक्ष हैं actor, block_kind, entered, event, holder, left, state, substate, और workstream
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

## 11. A window too narrow for the tree

At 40 columns the table falls back to the stacked form, one block per row. The
first three rows of block 2 come out like this.

```text
  Reference  intake
  Entity     state
  Title      Intake
  Count      4

  Reference  |-- ready
  Entity     substate
  Count      3
  Not shown  3 not shown

  Reference  |-- active
  Entity     substate
  Count      0
```

Nothing is lost and nothing overflows, so the tree is readable at a width where a
table would not be. What is lost is the shape: the guide characters travel as the
value of a field, and a reader has to reassemble the nesting from them rather than
see it. Whether the stacked form should know it is stacking a tree is a question
for the card `dinah-156`, and this card changes nothing in the shared machinery.

The earlier column shape did overflow here rather than stacking, because its first
column stood wider than the heading `Node` and the stacked fallback fires only
when a column stands at its own heading and a field overruns it. The heading
`Reference` is long enough that the fallback now fires. The weakness in the
predicate is still there and this tree no longer reaches it.
