# dinah-141 UX sketch: reading and writing a workbench's own fields

Two kinds of block appear below. A block headed **today** came out of a binary built from
commit `a62b6fb`, run in a scratch workbench created for this card, and it is the tool as it
stands. A block headed **proposed** is drawn by hand to this card's spec, because the
commands it shows do not exist yet.

The scratch workbench was created by running `dinah init` in a directory named
`Dinah development`, so its title is `Dinah development` and the slug `dinah init` derived
for it is `dinahdevelopment`.

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
the repair rather than standing blank.

```text
$ dinah workbench
  Field     Value
  --------  ------------------------------
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
the same mistake.

```text
$ dinah workbench get profile
dinah.unknown-key this workbench records no field called profile
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

Proposed. The first attempt refuses and shows you what the rename would do to a reference you
already know.

```text
$ dinah workbench set slug dinah-dev
dinah.unconfirmed the slug is the first half of every card reference in this workbench, so dinahdevelopment-1 becomes dinah-dev-1 and every reference written down elsewhere stops matching; run it again with --yes
```

Proposed. The second attempt carries the flag and Dinah renames.

```text
$ dinah workbench set slug dinah-dev --yes
$ dinah ls
  Card         Standing  Title
  -----------  --------  --------
  dinah-dev-1  ready     tmp card
```

Today, and unchanged by this card. `delete` refuses under the same name and keeps its own
sentence, because Dinah picks the sentence from the refusal name together with the command
that raised it.

```text
$ dinah delete wb-1
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
C:\dinah-scratch\dinah-141-spec\sandbox\Dinah development\.dinah\9675dcfddb9f\workbench.md

$ dinah path .
C:\dinah-scratch\dinah-141-spec\sandbox\Dinah development\.dinah\9675dcfddb9f\workbench.md
```

## 7. Help

Proposed. The new command's help block follows the shape every other command's does. The
order of the rows is the order Dinah checks them in, and it is the order the core profile's
five tiers put them in: the workbench-level operator check first, then whether what you named
exists, then whether what you typed is well formed, then whether you are entitled to write,
and last whether you confirmed. A read stops after row 2, because reading a field is open to
anybody.

```text
$ dinah help workbench
workbench [get|set] [field] [value] [--yes]

read this workbench's own fields, or write one

What can go wrong, in the order each is checked:
  Order  What can go wrong                             Refusal
  -----  --------------------------------------------  ------------------
  1      on a write, the workbench designates an operator
                                                       no-operator
  2      the field is one this workbench records       dinah.unknown-key
  3      the value is present and well formed          malformed
  4      on a write, the request names an owner        no-owner
  5      that owner is the operator                    not-operator
  6      a slug rename carries the confirmation flag   dinah.unconfirmed

Exit codes: 0 ok, 2 refused, 3 stale, 4 unreachable.
```

## 8. The slug a new workbench gets

Today. `dinah init` drops the space rather than replacing it, so the two words run together.

```text
$ mkdir "Dinah development" && cd "Dinah development"
$ dinah init
Workbench created at C:\dinah-scratch\dinah-141-spec\sandbox\Dinah development\.dinah\9675dcfddb9f.
$ dinah workbenches
  Workbench  Dinah development
  Slug       dinahdevelopment
  Path       C:\dinah-scratch\dinah-141-spec\sandbox\Dinah development\.dinah\9675dcfddb9f
```

Today. A slug you write by hand is refused if it carries a dash.

```text
$ dinah init --slug dinah-dev
malformed slug is missing, empty, or will not parse
```

Proposed. Dinah derives the readable form instead, and accepts a dashed slug you write
yourself.

```text
$ dinah init
Workbench created at C:\dinah-scratch\dinah-141-spec\sandbox\Dinah development\.dinah\9675dcfddb9f.
$ dinah workbenches
  Workbench  Dinah development
  Slug       dinah-development
  Path       C:\dinah-scratch\dinah-141-spec\sandbox\Dinah development\.dinah\9675dcfddb9f
```

Proposed. One shape of dashed slug is refused, because Dinah would read it as a card
reference. The reference parser splits on the last dash, so a slug whose own last segment is
nothing but digits reads as a prefix and a card number.

```text
$ dinah init --slug sprint-2
malformed a workbench slug may not end in a dash and digits, because Dinah reads sprint-2 as card 2
```

Proposed. A directory name that would derive such a slug is repaired rather than refused,
which is what `dinah init` already does with every other character a directory name carries
and the grammar does not.

```text
$ mkdir "Sprint 2" && cd "Sprint 2"
$ dinah init
$ dinah workbenches
  Workbench  Sprint 2
  Slug       sprint2
  Path       C:\dinah-scratch\dinah-141-spec2\sandbox\Sprint 2\.dinah\1f0c4b9d22a7
```

Today, and unchanged by this card. A dashed slug already resolves every reference correctly,
because the reference parser splits on the last dash and a card number is always the final
segment. This block was produced by writing a dashed slug into the anchor by hand.

```text
$ dinah ls
  Card         Standing  Title
  -----------  --------  -------------
  dinah-dev-1  ready     A first card
  dinah-dev-2  ready     A second card

$ dinah show dinah-dev-1
dinah-dev-1  A first card  [Intake / ready]

$ dinah path dinah-dev-1
C:\dinah-scratch\dinah-141-spec\sandbox\dashed\.dinah\7d4380dc8eae\cards\b51dd2700612\card.md
```
