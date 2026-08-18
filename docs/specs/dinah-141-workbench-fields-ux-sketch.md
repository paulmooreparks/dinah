# dinah-141 UX sketch: reading and writing a workbench's own fields

Two kinds of block appear below. A block headed **Today** came out of a binary built from
commit `a62b6fb`, which is the current head of `main`, run in a scratch workbench created
for this card, and it is quoted verbatim. A block headed **Proposed** is drawn by hand to
this card's spec, because the command it shows does not exist yet.

Every proposed table below was laid out through the product's own layout algorithm at the
eighty-column width an unbounded run measures against, and the algorithm was checked against
the real binary's output for `dinah help delete`, `dinah help move`, and `dinah help unblock`
first. So a column width, a rule length, and a wrapped row here are what the shipped command
prints rather than what a drawing guessed at.

The scratch workbench was created by running `dinah init` in a directory named
`Dinah development`, so its title is `Dinah development` and the slug `dinah init` derived
for it is `dinahdevelopment`. It holds two cards.

## 1. What a workbench reference reaches today

Today. Every spelling of a workbench reference refuses, and each refusal reports it as a
missing card.

```text
$ dinah path workbench
unknown-card this workbench carries no card workbench

$ dinah path .
unknown-card this workbench carries no card .

$ dinah edit workbench
unknown-card this workbench carries no card workbench
```

Today. One command already accepts the reference, because `attach` resolves through a
different function that has understood `workbench` all along.

```text
$ dinah attach workbench notes.txt
$ echo $?
0
```

## 2. The listing

Proposed. You run `dinah workbench` with no argument and Dinah prints the three fields the
workbench records about itself.

```text
$ dinah workbench
  Field     Value
  --------  -----------------
  title     Dinah development
  slug      dinahdevelopment
  operator  paul
```

Proposed. A workbench written before the slug field existed carries none, and the row names
the repair rather than standing blank. The cell comes from the helper `dinah states` and
`dinah workbenches` already use, so the three listings say the same thing about a missing
slug.

```text
$ dinah workbench
  Field     Value
  --------  -----------------------------------
  title     Dinah development
  slug      no slug (run check --migrate-slugs)
  operator  paul
```

## 3. Reading one field

Proposed. `get` prints the stored value alone, with no heading and no padding, so a script
can read it.

```text
$ dinah workbench get slug
dinahdevelopment

$ dinah workbench get title
Dinah development
```

Proposed. A field name outside the three refuses under the name `config` already uses for
the same mistake, and it renders the sentence `config` renders. That sentence gains the word
`field`, because one message now covers a user setting and a workbench field alike.

```text
$ dinah workbench get profile
dinah.unknown-key Dinah knows no setting or field called profile

$ dinah config get profile
dinah.unknown-key Dinah knows no setting or field called profile
```

## 4. Writing the title

Proposed. A title carrying spaces is one quoted word, exactly as `config set` and `add`
already require.

```text
$ dinah workbench set title "Dinah, the tool"
$ dinah workbench get title
Dinah, the tool
```

Proposed. You forget the quotation marks and Dinah tells you what to type instead, through
the message it already uses for every other free-text slot.

```text
$ dinah workbench set title Dinah, the tool
dinah.multiple-words Dinah read 3 separate words for the value, and it only accepts one. Put quotation marks around the whole thing: dinah workbench set title "Dinah, the tool"
```

Proposed. Dinah refuses to empty any of the three fields, because a workbench with no title
will not open at all and a workbench with no operator has no way to designate one again.

```text
$ dinah workbench set title ""
malformed title is missing, empty, or will not parse
```

## 5. Renaming the slug

Proposed. The first attempt refuses and tells you what the rename costs.

```text
$ dinah workbench set slug dinah-dev
dinah.unconfirmed Dinah renames every card in this workbench when you change the slug to dinah-dev, so every card reference you have written down elsewhere stops matching. Run the command again with --yes.
```

The sentence names the new slug and not the old one, and that is a limit of the mechanism
rather than a choice about what is worth saying. Dinah picks this sentence by pairing the
refusal name with the command that raised it, and the object the command hands the renderer
carries exactly one value for the sentence to fill. An earlier draft of this sketch drew
`dinahdevelopment-1 becomes dinah-dev-1`, which needs two.

Proposed. The second attempt carries the flag and Dinah renames.

```text
$ dinah workbench set slug dinah-dev --yes
$ dinah ls
  Card         Standing  Title
  -----------  --------  -------------
  dinah-dev-1  ready     tmp card
  dinah-dev-2  ready     a second card
```

Today. `delete` refuses under the same name and prints the sentence below, and this card
leaves that sentence and its eight translations untouched. Dinah reaches it by falling back
to the shared entry when the command that raised the refusal has added no sentence of its
own.

```text
$ dinah delete dinahdevelopment-1
dinah.unconfirmed delete destroys history, so it needs --yes
```

Today. The old reference still resolves after a rename, because a card reference resolves on
its number and a prefix left over from a rename identifies exactly one card. This block was
produced by editing the anchor by hand, which is the only rename available today.

```text
$ dinah show dinahdevelopment-1
dinah-dev-1  tmp card  [Intake / ready]
```

## 6. The plumbing commands reach the anchor

Proposed. `path` and `edit` accept the two spellings `attach` already accepts.

```text
$ dinah path workbench
C:\dinah-scratch\dinah-141-spec3\sandbox\Dinah development\.dinah\83dd4ec392da\workbench.md

$ dinah path .
C:\dinah-scratch\dinah-141-spec3\sandbox\Dinah development\.dinah\83dd4ec392da\workbench.md
```

## 7. Help

Proposed. The new command's help block follows the shape every other command's does. The
order of the rows is the order Dinah checks them in, and it is the order the core profile's
tiers put them in: whether what you named exists, then whether what you typed is well
formed, then whether you are entitled to write, and last whether you confirmed. A read stops
after row 1, because reading a field is open to anybody.

Dinah also checks that the workbench designates an operator, ahead of every row below, and
that row is not listed. No command outside the five the profile specifies lists a
workbench-level check in its help, so `delete` and `attach` leave the same row out of theirs.

```text
$ dinah help workbench
workbench [get|set] [field] [value] [--yes]

read this workbench's own fields, or write one

What can go wrong, in the order each is checked:
  Order  What can go wrong                            Refusal
  -----  -------------------------------------------  -----------------
  1      the field is one this workbench records      dinah.unknown-key
  2      the value is present and well formed         malformed
  3      on a write, the request names an owner       no-owner
  4      that owner is the operator                   not-operator
  5      a slug rename carries the confirmation flag  dinah.unconfirmed

Exit codes: 0 ok, 2 refused, 3 stale, 4 unreachable.
```

## 8. The slug a new workbench gets

Today. `dinah init` drops the space rather than replacing it, so the two words run together.

```text
$ mkdir "Dinah development" && cd "Dinah development"
$ dinah init
Workbench created at C:\dinah-scratch\dinah-141-spec3\sandbox\Dinah development\.dinah\83dd4ec392da.
$ dinah workbenches
  Workbench  Dinah development
  Slug       dinahdevelopment
  Path       C:\dinah-scratch\dinah-141-spec3\sandbox\Dinah development\.dinah\83dd4ec392da
```

Today. A slug you write by hand is refused if it carries a dash, whatever the dash is doing
in it.

```text
$ dinah init --slug dinah-dev
malformed slug is missing, empty, or will not parse

$ dinah init --slug sprint-2
malformed slug is missing, empty, or will not parse
```

Proposed. Dinah derives the readable form instead, and accepts a dashed slug you write
yourself.

```text
$ dinah init
Workbench created at C:\dinah-scratch\dinah-141-spec3\sandbox\Dinah development\.dinah\83dd4ec392da.
$ dinah workbenches
  Workbench  Dinah development
  Slug       dinah-development
  Path       C:\dinah-scratch\dinah-141-spec3\sandbox\Dinah development\.dinah\83dd4ec392da
```

Proposed. One shape of dashed slug stays refused, because Dinah would read it as a card
reference. The reference parser splits on the last dash, so a slug whose own last segment is
nothing but digits reads as a prefix and a card number. The refusal keeps the name it has
today and gains a clause naming the slug, spliced on the way the `malformed` refusal already
splices the file it was raised over.

```text
$ dinah init --slug sprint-2
malformed slug is missing, empty, or will not parse; Dinah reads sprint-2 as a card reference, so a workbench slug may not end in a dash and a number
```

Today, and unchanged by this card. A directory name that would derive such a slug is
repaired rather than refused, because `dinah init` already repairs every other character a
directory name carries and the grammar does not.

```text
$ mkdir "Sprint 2" && cd "Sprint 2"
$ dinah init
Workbench created at C:\dinah-scratch\dinah-141-spec3\sandbox\Sprint 2\.dinah\e7f838aa56d3.
$ dinah workbenches
  Workbench  Slug     Path
  ---------  -------  ----------------------------------------------------------
  Sprint 2   sprint2  C:\dinah-scratch\dinah-141-spec3\sandbox\Sprint 2\.dinah\e7f838aa56d3
```

Today, and unchanged by this card. A dashed slug already resolves every reference correctly,
because the reference parser splits on the last dash and a card number is always the final
segment. This block was produced by writing a dashed slug into the anchor by hand.

```text
$ dinah ls
  Card         Standing  Title
  -----------  --------  -------------
  dinah-dev-1  ready     tmp card
  dinah-dev-2  ready     a second card

$ dinah show dinah-dev-1
dinah-dev-1  tmp card  [Intake / ready]

$ dinah path dinah-dev-1
C:\dinah-scratch\dinah-141-spec3\sandbox\Dinah development\.dinah\83dd4ec392da\cards\ccb8c5921ea1\card.md
```

Today. Dinah opens a workbench whose stored slug is `sprint-2` without complaint, and
`dinah check` finds nothing to report. The reference then reaches a card instead of the
workbench, in silence, because a read carries no stale-prefix warning.

```text
$ dinah show sprint-2
sprint-2-2  a second card  [Intake / ready]
$ echo $?
0
$ dinah check
No structural defects found.
```

Proposed. `dinah check` reports that stored slug under a finding of its own, and the
workbench still opens, so the checker can say what the write path now refuses.
