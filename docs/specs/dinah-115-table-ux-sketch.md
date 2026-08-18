# dinah-115: what a table looks like before and after

Every "before" block below came out of the binary built from `f22c06b`, the
trunk head this spec was written from. Every "after" block came out of a harness
that implements the spec's width rules and was fed the same values, so the
columns land where the implementation will put them. Nothing here was aligned by
hand.

The fixture is a workbench in a directory named `dinah-115-play`, holding three
cards. One is held, one is blocked, one is ready. The three titles are English,
Japanese, and Hindi, because a Japanese title and a Hindi title measure
differently from their character counts and a table that gets them wrong gets
them wrong quietly.

## 1. The state listing, which is the case that was reported

Five columns of identifiers, slugs, names, kinds, and counts run across the line
with nothing saying which is which. The operator hit this and reported having to
look carefully to work out what he was being told, despite having designed the
tool, which is the report the card was filed on.

Before:

```
dinah-115-play  (C:\dinah-scratch\dinah-115-spec2\play\dinah-115-play\.dinah\7f882b2a4ece)  [search]
acting as spec-agent, operator: yes

  9e4458bda555  intake                  Intake                          intake    3
  2b1e63102b81  doing                   Doing                           work      0
  a6a4a85f7144  done                    Done                            done      0
```

Each of those rows is 90 columns long. The last glyph a reader can see starts at
display column 82, and seven spaces follow it to the end of the line.

After:

```
dinah-115-play  (C:\dinah-scratch\dinah-115-spec2\play\dinah-115-play\.dinah\7f882b2a4ece)  [search]
acting as spec-agent, operator: yes

  Identifier    Slug    Name    Kind    Cards  Moved by
  ------------  ------  ------  ------  -----  --------
  9e4458bda555  intake  Intake  intake  3      agent
  2b1e63102b81  doing   Doing   work    0      agent
  a6a4a85f7144  done    Done    done    0      agent
```

The row is now 55 columns and its last glyph starts at 47. Three things changed
and they all follow from measuring the rows instead of declaring the columns at
the call site: the headings appear, the padding between columns closes up, and
the trailing run of spaces goes. That trailing run is invisible on screen. It
shows up in a diff, in a pasted transcript, and in a defect report, which is
where it costs somebody time.

The third column used to be called nothing at all, and the obvious word for it
is wrong. Every row in this table is a state, so a heading reading `State` names
the table rather than the column. What the column holds is the state's display
name, as against the slug beside it, so the heading is `Name`.

The last column used to carry a mark that appeared on an operator-owned state
and left a blank cell everywhere else. It now says who may move a card out of
the state, in every row, reading `agent` or `operator`. A blank cell is
indistinguishable from a rendering fault, and the tree already argues this way:
`slugCell` in `render.go` prints a placeholder naming the repair rather than
padding an empty string, and its comment says exactly that.

## 2. The same listing without the column of identifiers

Whether that column stays is an open question this card carries, and the spec
recommends keeping it. Here is what dropping it would look like, so the ruling
can be given on a picture rather than on a description.

```
  Slug    Name    Kind    Cards  Moved by
  ------  ------  ------  -----  --------
  intake  Intake  intake  3      agent
  doing   Doing   work    0      agent
  done    Done    done    0      agent
```

The block loses fourteen columns off its left edge. Every remaining column stays
where it was relative to the others. What a reader loses is the state's internal
identity, which `--json` still carries and which the workbench on disk still
uses for its directory names.

## 3. The card listing, where a declared width was already too narrow

Before:

```
  dinah115play-1
                active    Wire the export path through the interchange form
  dinah115play-2
                blocked   研究テーブルの見出し
  dinah115play-3
                ready     हिन्दी शीर्षक की जाँच
```

Three cards, six lines, and no column anybody can scan down. The card reference
column is declared fourteen columns wide and these references draw exactly
fourteen, which is enough to trip the renderer's overflow rule, since a field
that reaches its column takes the rest of the line rather than sitting in it.

After:

```
  Card            Standing  Title
  --------------  --------  -------------------------------------------------
  dinah115play-1  active    Wire the export path through the interchange form
  dinah115play-2  blocked   研究テーブルの見出し
  dinah115play-3  ready     हिन्दी शीर्षक की जाँच
```

A workbench takes its slug from its directory name, and the slug decides how
wide a card reference draws, so no number written into the source is right for
every workbench. A measured column is right for all of them.

## 4. The settings listing, and what a narrow window does to it

Before, at `COLUMNS=80`:

```
  lang        en                      default
  actor       spec-agent              environment
  editor      notepad                 fallback
  workbench   C:\dinah-scratch\dinah-115-spec2\play\dinah-115-play\.dinah\7f882b2a4ece
                                      search
```

After, at `COLUMNS=80`:

```
  Setting    Value       Source
  ---------  ----------  -----------
  lang       en          default
  actor      spec-agent  environment
  editor     notepad     fallback
  workbench  C:\dinah-scratch\dinah-115-spec2\play\dinah-115-play\.dinah\7f882b2a4ece
                         search
```

The path draws 70 columns and cannot be laid out inside an 80-column window
whatever the columns are, so it does not get to widen the value column. It takes
the rest of its own line and the source beside it resumes underneath, in the
shape the row renderer already ships. The other three rows close up.

At `COLUMNS=60` this block is byte for byte the same, because the path was
already out of the measurement and the remaining columns already fit. Narrow the
window to 40 and the backstop starts work:

```
  Setting  Value    Source
  -------  -------  -----------
  lang     en       default
  actor    spec-agent
                    environment
  editor   notepad  fallback
  workbench
           C:\dinah-scratch\dinah-115-spec2\play\dinah-115-play\.dinah\7f882b2a4ece
                    search
```

The value column has been narrowed from 10 to 7, which is as far as it can go
without eating its own heading, so `spec-agent` now overflows and takes its own
line. Nothing is truncated at any width and the path is still copyable.

## 5. The command list, which does not move

Measurement could have gone badly here and did not. Two of the twenty-nine
commands carry a syntax line far longer than the rest, and a column that simply
took the widest value it saw would have widened by 37 columns to hold them.

The measured width of the syntax column is 37, the gutter is 2, and the indent
is 2, so every summary starts at display column 41. That is where the declared
width of 39 has always started it. The block is unchanged apart from its
heading:

```
  Command                                What it does
  -------------------------------------  ---------------------------------------
  add <title> [--state <state>]          file a new card in the first state
  claim <card> [--expires <duration>]    take up a ready card
  move <card> <state> [--override]       carry a card to another state
```

Four rows do not fit an 80-column window when their fields are packed tight. Two
of them are over on account of their syntax, and those two are the ones that
matter, because the syntax is the field that would otherwise widen the column.
They still continue on a line of their own:

```
  init [--from <source>] [--slug <slug>] [--operator <actor>]
                                         create a workbench here, optionally from a template
  check [--finish] [--migrate-ordinals] [--migrate-slugs] [--migrate-states]
                                         look for structural defects in this workbench
```

The other two are `archive` and `delete`. They are over because their summaries
are long rather than their syntax, and since the summary is the last field of
the row and the last field of a row is never a candidate for the drop, what gets
dropped from their measurement is the syntax beside it, which draws 13 and 18
columns respectively and was never going to widen a column already sitting at
37. Nothing about those two rows moves.

The global flag list is a table of its own with its own heading. Its summaries
move one column left, from 22 to 21, because the widest flag draws 17 and the
declared width was 20:

```
  Option             What it does
  -----------------  ----------------------------------------------------------
  --workbench <dir>  use this workbench instead of the one discovered from here
  --json             emit the canonical machine form
  --quiet            suppress served instructions on claim and move
  --lang <tag>       render in this language
  --actor <name>     act as this owner
```

## 6. The rest of the inventory

```
$ dinah next
  State   Card            Title
  ------  --------------  ------------------
  Intake  dinah115play-3  हिन्दी शीर्षक की जाँच
  Doing   nothing ready
  Done    nothing ready

$ dinah log dinah115play-1
  When                  Action     Actor       Detail
  --------------------  ---------  ----------  ---------------------------------
  2026-08-18T09:07:27Z  created    spec-agent  Wire the export path through the interchange form
  2026-08-18T09:08:51Z  claimed    spec-agent
  2026-08-18T09:08:51Z  commented  spec-agent

$ dinah guide
  Topic             Title
  ----------------  -----------------------------------
  getting-started   Getting started
  verbs             The five verbs
  workbench-layout  What a workbench looks like on disk

$ dinah instructions dinah115play-3
  State  Name   Direction
  -----  -----  ---------
  doing  Doing  forward
  done   Done   forward

$ dinah version --catalogs
  Language  Translated
  --------  ----------
  en        244/244
  af        0/244
  cs        0/244
  de        0/244
  es        0/244
  fil       0/244
  hi        244/244
  id        0/244

$ dinah help add
  Order  What can go wrong                              Refusal
  -----  ---------------------------------------------  -------------
  1      the request carries a title                    malformed
  2      the named state is one the workbench declares  unknown-state
  3      the named state is below its capacity limit    at-capacity
```

Look at the two states offering nothing under `dinah next`. A row is allowed to
stop short, and the field it stops on takes the rest of the line and widens no
column, so `nothing ready` sits in the card column without pushing the title
column right.

## 7. The same block in Hindi

Headings come out of the message catalog, so a language whose words run longer
widens the columns holding them and touches nothing else. The Hindi check
sentences measure shorter than their English counterparts, and the refusal
column moves left to follow:

```
$ dinah help add --lang hi
  Order  What can go wrong               Refusal
  -----  ------------------------------  -------------
  1      निवेदन में शीर्षक है                 malformed
  2      नामित स्थिति वर्कबेंच घोषित करती है  unknown-state
  3      नामित स्थिति अपनी सीमा से नीचे है   at-capacity
```

Two things about that block are worth saying. The Hindi heading words are not
written yet, which is one of the questions the card carries, so the headings
above fall back to English while the rows print in Hindi. And the block is
aligned by the measure rather than by eye: every refusal name begins at display
column 41, counted in the columns a terminal gives Devanagari rather than in
characters. A font that draws a combining mark its own way will show something
else, and that is the terminal disagreeing with the standard rather than the
table drifting.

## 8. The separator

A separator under the headings is required and it carries the job in every
language. Capitalisation is what English adds on top of it. Most of the
languages this project ships have no case at all, so the rule cannot be about
capitals, and each catalog decides for itself how its headings read as headings.

The separator is one rule for each column and the gaps between the rules stay,
so the rules never join into a single line running across the heading row. A
reader sees the columns as separate things because of those gaps. Each rule is
exactly as wide as the column above it, counted in screen columns and never
typed. This is the shape a separator must never take:

```
  Identifier    Slug    State   Kind    Cards
  ---------    ----    -----   ----    -----
  1ad331cffbff  intake  Intake  intake  3
```

Those rules are as wide as the words above them rather than as wide as their
columns, so the third rule starts one column left of the third heading and
nothing below them lines up. That is this card's own defect, reappearing inside
the fix for it.

Here is the card listing drawn the way the operator ruled:

```
  Card            Standing  Title
  --------------  --------  -------------------------------------------------
  dinah115play-1  active    Wire the export path through the interchange form
  dinah115play-2  blocked   研究テーブルの見出し
  dinah115play-3  ready     हिन्दी शीर्षक की जाँच
```

The rule under `Title` draws forty-nine because the widest thing in that column
draws forty-nine. A last column used to have no width at all, since it takes the
rest of the line rather than being padded to anything, and it now carries one:
the longest of its heading and its values, measured the way every other column
is measured. That width reaches the rule and nothing else. The fields under it
are still printed unpadded, so no row picks up a trailing run of spaces.

The heading counts as content of its column, which matters where a heading is
the widest thing in the column. `Standing` draws eight over values drawing six,
and a width taken from the values alone would put a six-column rule under an
eight-column word. The same block on the widest table the tool prints:

```
  Identifier    Slug    Name    Kind    Cards  Moved by
  ------------  ------  ------  ------  -----  --------
  9e4458bda555  intake  Intake  intake  3      agent
  2b1e63102b81  doing   Doing   work    0      agent
  a6a4a85f7144  done    Done    done    0      agent
```

A rule stops at the right edge of the display. The column of summaries in the
command list measures seventy-three, its column starts at display column 41, and
an eighty-column window leaves thirty-nine, so the rule draws thirty-nine while
the summaries themselves run on as they always have:

```
  Command                                What it does
  -------------------------------------  ---------------------------------------
  add <title> [--state <state>]          file a new card in the first state
  claim <card> [--expires <duration>]    take up a ready card
  archive <ref>                          move a card, a state, or anything below a card, out of the live set
  delete <ref> --yes                     destroy a card, a state, or anything below a card, along with its history
```

The clamp shortens a rule and changes no other line. A heading is where it was,
a value the window cannot hold takes the rest of its own line in the shape the
row renderer already ships, and a single unbroken value with nowhere to wrap
runs past the edge with its rule stopping short of it. The `dinah log` block in
section 6 is the other place the clamp binds today, where a detail column
measuring forty-nine starts at display column 47 and draws a rule of
thirty-three.

## 9. Piped output

Headings and separators print when output goes to a file or a pipe, and the
widths are chosen against a window of 80 when nothing states one.

```
$ dinah states > states.txt
$ cat states.txt
  Identifier    Slug    Name    Kind    Cards  Moved by
  ------------  ------  ------  ------  -----  --------
  9e4458bda555  intake  Intake  intake  3      agent
```

The alternative is to drop them when nobody appears to be watching. Then the
bytes a person sees would depend on whether they redirected, and a transcript
pasted into a defect report would differ from what its author saw, with nothing
on either copy saying so. `--json` is the surface a program reads and it never
grew a heading.

## 10. The words a reader sees

Every column names its value by what a reader does with it, in the reader's
words, never by the field's name in the code. Where the value is something a
person types on the command line, the heading is the word the command line uses
for it. Where the value is an internal identifier nobody types, the heading says
`Identifier` and claims nothing more.

| Table | Headings |
|---|---|
| `states`, and the same block inside `status` | Identifier, Slug, Name, Kind, Cards, Moved by |
| the cards you hold, inside `status` | Card, Title |
| the blocked cards, inside `status` | Card, Reason |
| `ls` | Card, Standing, Title |
| `next` | State, Card, Title |
| `log` | When, Action, Actor, Detail |
| the links of `show` | Link, Card |
| the comments of `show` | When, Who |
| `config` | Setting, Value, Source |
| `workbenches`, and the ambiguous-workbench refusal | Workbench, Slug, Path |
| `version --catalogs` | Language, Translated |
| `guide` | Topic, Title |
| the moves of `instructions` | State, Name, Direction |
| the command list of bare `dinah` | Command, What it does |
| the flag list of bare `dinah` | Option, What it does |
| `help <command>` | Order, What can go wrong, Refusal |
| the slugs `check --migrate-slugs` assigned | Slug, Title |

Forty-six headings in all. Two blocks print a single column and take no heading
and no separator: the findings of `check`, and the stranded states
`check --migrate-states` removed. One column under a sentence that already names
it is a list.

The word most worth a second look is `Moved by`. It heads the column that now
reads `agent` or `operator` on every row of the state listing, and it is the one
heading in the table above naming a question rather than a value. `Owner` and
`Who moves it` are the alternatives.
