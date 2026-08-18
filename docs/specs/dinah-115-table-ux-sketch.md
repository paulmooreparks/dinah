# dinah-115: what a table looks like before and after

Every "before" block below was printed by the binary built from `f22c06b`, the
trunk head this spec was written from. Every "after" block was computed from the
same values by the width rules the spec states, so the columns land where the
implementation will put them rather than where a hand alignment put them.

The fixture is a workbench holding three cards, one held, one blocked, one in
each of three scripts. Nothing about it is unusual except the card titles, which
are there because a Japanese title and a Hindi title measure differently from
their character counts.

## 1. The status listing, which is the case that was reported

A reader meets five columns of identifiers, slugs, names, kinds, and counts with
nothing saying which is which.

Before:

```
dinah-115-play  (C:\dinah-scratch\dinah-115-play\.dinah\aa1af304e624)  [search]
acting as spec-agent, operator: yes

  1ad331cffbff  intake                  Intake                          intake    3
  875c58408c3a  doing                   Doing                           work      0
  d634a2f5e819  done                    Done                            done      0
```

After:

```
dinah-115-play  (C:\dinah-scratch\dinah-115-play\.dinah\aa1af304e624)  [search]
acting as spec-agent, operator: yes

  identifier    slug    state   kind    cards
  1ad331cffbff  intake  Intake  intake  3
  875c58408c3a  doing   Doing   work    0
  d634a2f5e819  done    Done    done    0
```

Two things changed and both come from the same cause. The headings appear, and
the block lost thirty columns of padding, because the widths are now measured
from the rows rather than declared at the call site. The declared widths were
24 for the slug and 32 for the title, and no state in this workbench comes close
to either.

Each "before" row also ends in the padding of its last column, which the after
form drops. That trailing run is invisible on screen and shows up in a diff, in
a copied transcript, and in a defect report.

## 2. The card listing, where a declared width was already too narrow

Before:

```
  dinah115play-1
                ready     Wire the export path through the interchange form
  dinah115play-2
                ready     研究テーブルの見出し
  dinah115play-3
                ready     हिन्दी शीर्षक की जाँच
```

The card reference column is declared 14 wide and these references draw 15, so
every row breaks in two. A reader gets six lines for three cards and no column
they can scan down.

After:

```
  card            standing  title
  dinah115play-1  active    Wire the export path through the interchange form
  dinah115play-2  blocked   研究テーブルの見出し
  dinah115play-3  ready     हिन्दी शीर्षक की जाँच
```

The slug a workbench gets from its directory name decides how wide a card
reference is, so no declared number can be right for every workbench. A measured
column is right for all of them.

## 3. The settings listing, and what a narrow window does to it

Before, in a window wide enough:

```
  lang        en                      default
  actor       spec-agent              environment
  editor      notepad                 fallback
  workbench   C:\dinah-scratch\dinah-115-play\.dinah\aa1af304e624
                                      search
```

After:

```
  setting    value                                                source
  lang       en                                                   default
  actor      spec-agent                                           environment
  editor     notepad                                              fallback
  workbench  C:\dinah-scratch\dinah-115-play\.dinah\aa1af304e624  search
```

The path fits its column now, so the row that used to break no longer does.

Before, at `COLUMNS=60`:

```
  lang        en                      default
  actor       spec-agent              environment
  editor      notepad                 fallback
  workbench   C:\dinah-scratch\dinah-115-play\.dinah\aa1af304e624
                                      search
```

After, at `COLUMNS=60`:

```
  setting    value       source
  lang       en          default
  actor      spec-agent  environment
  editor     notepad     fallback
  workbench  C:\dinah-scratch\dinah-115-play\.dinah\aa1af304e624
                         search
```

The window is too narrow to hold the path, so the path does not get to widen the
value column. The other three rows close up, and the path takes the rest of its
own line in the shape the row renderer already gives it. Nothing is truncated at
any width.

## 4. The command list, which changes almost not at all

This block is the one where measurement could have gone wrong, and it is worth
reading closely. Two of the twenty-nine commands carry a syntax line far longer
than the rest, and a column measured naively would have widened by thirty
columns to hold them.

Before, and after, are the same layout:

```
WORK
  add <title> [--state <state>]          file a new card in the first state
  claim <card> [--expires <duration>]    take up a ready card
```

The measured width of the syntax column is 37 and the gutter is 2, which starts
every summary at display column 41, which is where the declared width of 39 has
always started it. The two long entries still continue on a line of their own:

```
  init [--from <source>] [--slug <slug>] [--operator <actor>]
                                         create a workbench here, optionally from a template
```

The one visible change is the heading, printed once above the first group rather
than once per group:

```
Usage: dinah <command> [arguments]

  command                                what it does

WORK
  add <title> [--state <state>]          file a new card in the first state
```

The global flag list is a table of its own and gets its own heading, `option`
and `what it does`. Its summaries move two columns left, from 22 to 21, because
the widest flag draws 17 and the declared width was 20.

## 5. What each remaining table looks like after

These are the rest of the inventory, computed the same way.

```
$ dinah next
  state   card            title
  Intake  dinah115play-1  Wire the export path through the interchange form
  Doing   nothing ready
  Done    nothing ready

$ dinah log dinah115play-1
  when                  action     actor       detail
  2026-08-18T08:25:52Z  created    spec-agent  Wire the export path through the interchange form
  2026-08-18T08:26:01Z  claimed    spec-agent
  2026-08-18T08:26:01Z  commented  spec-agent

$ dinah workbenches
  workbench       slug   path
  beta-workbench  beta   C:\dinah-scratch\amb-home\.dinah\0271ef99ee7a
  alpha           alpha  C:\dinah-scratch\amb-home\.dinah\a3e17a0501dd

$ dinah version --catalogs
  language  translated
  en        244/244
  af        0/244
  cs        0/244
  de        0/244
  es        0/244
  fil       0/244
  hi        244/244
  id        0/244

$ dinah guide
  topic             title
  getting-started   Getting started
  verbs             The five verbs
  workbench-layout  What a workbench looks like on disk

$ dinah instructions dinah115play-3
  state  name   direction
  doing  Doing  forward
  done   Done   forward

$ dinah help add
  order  what can go wrong                              refusal
  1      the request carries a title                    malformed
  2      the named state is one the workbench declares  unknown-state
  3      the named state is below its capacity limit    at-capacity
```

A row may stop short, as the two states offering nothing do above. The field it
stops on takes the rest of the line and widens no column.

## 6. The same table in Hindi

Headings are catalog entries, so a language whose words run longer widens the
columns that hold them and nothing else. The Hindi check sentences below measure
shorter than their English counterparts, and the refusal column moves left to
follow them.

```
$ dinah help add --lang hi
  order  what can go wrong               refusal
  1      निवेदन में शीर्षक है                 malformed
  2      नामित स्थिति वर्कबेंच घोषित करती है  unknown-state
  3      नामित स्थिति अपनी सीमा से नीचे है   at-capacity
```

The Hindi heading words are not written yet, which is the open question the card
carries. Until they are, this table prints its headings in English while its rows
print in Hindi, because a catalog falls back per key rather than per file.

## 7. Piped output

The headings print when output goes to a file or a pipe, and the widths are
chosen against a width of 80 when nothing states one.

```
$ dinah states > states.txt
$ cat states.txt
  identifier    slug    state   kind    cards
  1ad331cffbff  intake  Intake  intake  3
```

The alternative is to drop the headings when nobody is watching, which would
make the bytes a person sees depend on whether they redirected. A transcript
pasted into a defect report would then differ from what its author saw on
screen. `--json` is the surface a program reads, and it never grew a heading.

## 8. The heading style, and the one rejected alternative

The heading is a plain row in the table's own columns, with nothing under it.

```
  card            standing  title
  dinah115play-1  active    Wire the export path through the interchange form
```

A rule under the headings was considered and is not proposed:

```
  card            standing  title
  --------------  --------  -----
  dinah115play-1  active    Wire the export path through the interchange form
```

The rule costs a line on every table, and Dinah's tables are short. Three lines
of dashes above a three-row listing spend a third of the block on decoration. It
also has to decide what the last column's rule is as wide as, which is a
question the plain form never asks.

## 9. The words a reader sees

Every column names its value by what a reader does with it. The one this card
was filed over is the state identifier: twelve hexadecimal characters mean
nothing to a reader, and the honest heading for them is `identifier` rather than
a word that pretends they are something friendlier.

| Table | Headings |
|---|---|
| `states`, and the same block inside `status` | identifier, slug, state, kind, cards, note |
| the cards you hold, inside `status` | card, title |
| the blocked cards, inside `status` | card, reason |
| `ls` | card, standing, title |
| `next` | state, card, title |
| `log` | when, action, actor, detail |
| the links of `show` | link, card |
| the comments of `show` | when, who |
| `config` | setting, value, source |
| `workbenches`, and the ambiguous-workbench refusal | workbench, slug, path |
| `version --catalogs` | language, translated |
| `guide` | topic, title |
| the moves of `instructions` | state, name, direction |
| the command list of bare `dinah` | command, what it does |
| the flag list of bare `dinah` | option, what it does |
| `help <command>` | order, what can go wrong, refusal |
| the slugs `check --migrate-slugs` assigned | slug, title |

Two blocks print one column each and take no heading at all: the findings of
`check`, and the stranded states `check --migrate-states` removed. A single
column under a sentence that already names it is a list rather than a table.

The weakest word in the table above is `note`, which heads the column carrying
the `operator-owned` mark in the state listing. It is a column that is usually
empty, and the mark says what it means on its own, so the heading has little
work to do and no obvious word to do it with.
