# Dinah quick start

This guide walks one workbench, the folder of plain files that holds a board's
work, from an empty directory to a finished card, and every command the binary
offers turns up somewhere along the way. Every transcript below is real output
from `dinah 0.1.0`. The absolute paths in the output name the directories the
session ran in, written here as `C:\work\...` and `C:\Users\ana\...`, and
nothing else in the output is edited. A few blocks show a file's contents rather
than a transcript; those are what you type into the file, and they say so where
they appear.

Read it in order the first time. After that, the section headings are the index.

## Shells

The command lines below run unchanged in bash, zsh, and PowerShell, because they
use only plain arguments, relative paths, `mkdir`, and `cd`, and because they
leave it to Dinah to find the workbench by walking up from the working
directory. Two things do genuinely differ between those shells: setting an
environment variable, and substituting one command's output into another. Each
appears once below as a labeled pair. One further block uses a POSIX-only
utility, and it too is labeled where it appears.

The leading `$` marks a command line and is not part of the command. This
session ran on Windows. The paths in the output are therefore Windows paths; on
macOS or Linux the same runs print POSIX paths.

## What the binary tells you about itself

```
$ dinah version
dinah 0.1.0
conforms to dinah-core/1.0
storage format 1
[exit 0]
```

Those lines carry three different facts. The first is the build. The second
names the shared rule set this build follows; any other tool built to those same
rules can read this workbench and reach the same answers about it. The third
names the on-disk format.

`dinah help` lists every command, twenty-nine of them, in the four groups the
binary sorts them into. The flag spelling you may reach for first is not one it
accepts:

```
$ dinah --help
dinah.usage --help was not understood; run dinah help for the surface
[exit 2]
```

That refusal calls the list of commands the surface. The list is all the word
means. Running `dinah` with no arguments prints it too. For one command's
arguments and the reasons it can say no, ask for that command by name with
`dinah help move`.

## Open a workbench

A workbench is a directory of plain-text files. Create one where the work is,
and put it under version control alongside the project it belongs to.

```
$ mkdir release-notes
$ cd release-notes

$ dinah init --slug rel --operator ana
Workbench created at C:\work\release-notes.
[exit 0]
```

The slug is the prefix every card reference carries. The first card you file
here will therefore be `rel-1`. Leave `--slug` out and Dinah derives one from
the directory name. The operator is the owner who answers for the workbench,
and because only the operator can lift a block or force a move past a limit, a
workbench with nobody in that seat has acts nobody can perform. Leave
`--operator` out and Dinah records whoever you are acting as.

That call wrote `workbench.md`, a `states/` directory holding one file per
state, and a `.gitignore` that keeps the tool's lock files out of your commits.
Nothing else exists yet.

Every command from here on runs inside the workbench directory. None of them
needs a path, because Dinah finds the workbench by walking up from wherever you
are, the way git finds a repository. When that climb finds nothing, one more
place is tried: the `.dinah` directory in your home. A workbench that belongs to
you rather than to a project can live there. `DINAH_HOME` moves that directory
somewhere else, and the section on working from outside a workbench comes back
to it.

## Say who you are

Every act Dinah records carries an owner. The tool will not invent one.

```
$ dinah whoami
no-owner no owner was resolvable; set one with --actor, DINAH_ACTOR or config actor
[exit 2]
```

The refusal names the three places it looked, in the order it looked: the
`--actor` flag wins, then the `DINAH_ACTOR` environment variable, then your user
config. Set the last of those three once and forget it:

```
$ dinah config set actor ana
[exit 0]

$ dinah config get actor
ana
[exit 0]

$ dinah whoami
ana, operator: yes
[exit 0]
```

`whoami` answers two questions at once. The second matters because operator
standing governs what you are allowed to do. The settings live in `config.md`
under `.dinah` in your home directory, and because they belong to you rather
than to the workbench, they follow you to every workbench you work.

Asked for nothing in particular, `config` lists every setting it knows, what it
is currently resolving to, and which rung of the ladder answered:

```
$ dinah config
  lang        en                      default
  actor       ana                     config
  editor      notepad                 fallback
[exit 0]
```

Reading that third column saves an argument with yourself later. Here `lang`
came from nothing you set, `actor` came from the file you just wrote, and
`editor` is a program Dinah found rather than one you chose. An environment
variable shows up the same way, so a value you cannot account for names its own
source:

```
$ dinah config
  lang        en                      default
  actor       bo                      environment
  editor      notepad                 fallback
[exit 0]
```

The keys this version knows are `actor`, `lang`, and `editor`, and nothing else
is accepted:

```
$ dinah config set colour green
dinah.unknown-key this tool knows no setting called colour
[exit 2]
```

Dinah refuses to set that key, but it does not throw away keys it does not
recognise when it writes the file. Anything you added there by hand stays.

## Look at the flow

```
$ dinah states
  1d2f2a07a38b  intake                  Intake                          intake    0
  df8cd3d7f024  doing                   Doing                           work      0
  acd3a55081b7  done                    Done                            done      0
[exit 0]
```

Each row gives a state's identifier, its slug, its title, its kind, and how many
cards stand in it. The kinds are `intake`, `work`, and `done`. Because the flow
runs in the order `workbench.md` lists them, a move to a later state is forward
and a move to an earlier one is backward. Identifiers are generated per
workbench, so yours will differ from the ones printed here.

The slug is the short name you type. Every command that takes a state accepts
the identifier, the slug, or the title, and the match ignores case. Slugs are
derived from the titles when the workbench is created, so a workbench made
before the field existed carries none, and `dinah check --migrate-slugs` fills
them in.

Run `status` when you sit down. It prints that same list and adds what you
personally hold.

```
$ dinah status
release-notes  (C:\work\release-notes)
acting as ana, operator: yes

  1d2f2a07a38b  intake                  Intake                          intake    0
  df8cd3d7f024  doing                   Doing                           work      0
  acd3a55081b7  done                    Done                            done      0
[exit 0]
```

## Edit the workbench by hand

The files are the interface. Open `workbench.md`, give the workbench a real
title, and write the standing instructions below the settings block at the top,
the part between the `---` lines. Dinah's own messages call that block the
frontmatter. This block is the file, not a transcript:

```
---
format: 1
profile: dinah-core/1.0
title: Release 0.2
slug: rel
operator: ana
states:
  - 1d2f2a07a38b
  - df8cd3d7f024
  - acd3a55081b7
---
Every card on this workbench ends with a line in the changelog.
```

Then open the `state.md` of one state and do the same. A `wip_limit` caps how
many cards that state will hold:

```
---
wip_limit: 1
title: Doing
slug: doing
kind: work
---
Work the card until it is finished or until something stops you.
Leave a comment saying what you did before you carry it on.
```

Nothing copies that prose anywhere. Dinah serves it from where you wrote it, so
an edit reaches every reader at once. Check your work whenever you have been
editing by hand:

```
$ dinah check
No structural defects found.
[exit 0]

$ dinah status
Release 0.2  (C:\work\release-notes)
acting as ana, operator: yes

  1d2f2a07a38b  intake                  Intake                          intake    0
  df8cd3d7f024  doing                   Doing                           work      0/1
  acd3a55081b7  done                    Done                            done      0
[exit 0]
```

The limit shows up as `0/1` in the count column. To read what a position serves
without touching a card, ask for it:

```
$ dinah instructions doing

Instructions, this workbench:
Every card on this workbench ends with a line in the changelog.

Instructions, this state:
Work the card until it is finished or until something stops you.
Leave a comment saying what you did before you carry it on.
[exit 0]
```

Give `dinah instructions` a card reference instead and it serves the position
that card is standing at.

## File some cards

```
$ dinah add "Write the release notes"
rel-1  Write the release notes  [Intake / ready]
[exit 0]

$ dinah add "Draft the changelog"
rel-2  Draft the changelog  [Intake / ready]
[exit 0]

$ dinah add "Check the download links" --state doing
rel-3  Check the download links  [Doing / ready]
[exit 0]
```

A card lands in the first state of the flow unless `--state` says otherwise, and
it arrives with substate `ready`. Anybody may pull a ready card. The bracket
after each title is the card's whole position: the state it stands in, then its
substate.

To read the board back, `ls` lists cards and takes an optional state and an
optional `--ready` filter, `next` reports what each state is currently offering,
and `show` prints one card. Listings run in queue order, oldest arrival first,
and cards that arrived inside the same second fall back to the order they were
filed in. That order does not depend on how fast you type.

```
$ dinah ls
  rel-1         ready     Write the release notes
  rel-2         ready     Draft the changelog
  rel-3         ready     Check the download links
[exit 0]

$ dinah ls doing
  rel-3         ready     Check the download links
[exit 0]

$ dinah ls intake --ready
  rel-1         ready     Write the release notes
  rel-2         ready     Draft the changelog
[exit 0]

$ dinah next
  Intake                          rel-1         Write the release notes
  Doing                           rel-3         Check the download links
  Done                            nothing ready
[exit 0]

$ dinah next doing
  Doing                           rel-3         Check the download links
[exit 0]

$ dinah show rel-1
rel-1  Write the release notes  [Intake / ready]
[exit 0]
```

## The five commands that move a card

Five commands change where a card stands: `claim`, `move`, `release`, `block`,
and `unblock`. Dinah's own guide calls these five the verbs; read it with
`dinah guide verbs`. What each one does is fixed by the shared rules; a second
tool reading the same workbench answers the same way.

Work is taken rather than handed out, so you claim your own card:

```
$ dinah claim rel-1
rel-1  Write the release notes  [Intake / active]
  held by ana

Instructions, this workbench:
Every card on this workbench ends with a line in the changelog.

Moves this card may make:
  df8cd3d7f024  Doing                           forward
  acd3a55081b7  Done                            forward
[exit 0]
```

A successful claim serves you the instructions of the position and the moves the
flow allows from it. You do not have to remember either. Pass `--quiet` when you
have read them already.

A held card is not available to anybody else:

```
$ dinah claim rel-1 --actor bo
held ana holds this card
[exit 2]
```

`move` carries a card to another state and changes nothing else; a holder who
moves a card still holds it. `Doing` is capped at one and already has `rel-3` in
it, so the move below is refused:

```
$ dinah move rel-1 doing
at-capacity state df8cd3d7f024 has reached its limit
[exit 2]
```

The override is the operator's alone, and it is recorded on the move:

```
$ dinah move rel-1 doing --override
rel-1  Write the release notes  [Doing / active]
  held by ana

Instructions, this workbench:
Every card on this workbench ends with a line in the changelog.

Instructions, this state:
Work the card until it is finished or until something stops you.
Leave a comment saying what you did before you carry it on.

Moves this card may make:
  1d2f2a07a38b  Intake                          backward
  acd3a55081b7  Done                            forward
[exit 0]
```

Say what you did while you are there:

```
$ dinah comment rel-1 "Drafted entries for the four merged branches."
rel-1  Write the release notes  [Doing / active]
  held by ana
[exit 0]

$ dinah comment rel-1 "Second half needs the signing certificate."
rel-1  Write the release notes  [Doing / active]
  held by ana
[exit 0]
```

Pass a single dash instead of the text and `comment` reads standard input. A
script uses that to hand it something longer than a command line wants.

When something stops the work, say so on the card. A block frees the card and
records why. That is what turns an obstacle into something visible:

```
$ dinah block rel-2 "Waiting on the signing certificate" --kind external
rel-2  Draft the changelog  [Intake / blocked]
  blocked: Waiting on the signing certificate
[exit 0]
```

The reason is required and it is prose, because the things that stop real work
vary, while the `--kind` is a short label of your own choosing for grouping
later. Blocks show up in `status`:

```
$ dinah status
Release 0.2  (C:\work\release-notes)
acting as ana, operator: yes

  1d2f2a07a38b  intake                  Intake                          intake    1
  df8cd3d7f024  doing                   Doing                           work      2/1
  acd3a55081b7  done                    Done                            done      0

You are holding:
  rel-1         Write the release notes

Blocked, waiting on the operator:
  rel-2         Waiting on the signing certificate
[exit 0]
```

Lifting a block is the operator's act, because an obstacle raised is an obstacle
handed to whoever answers for the workbench:

```
$ dinah unblock rel-2 --actor bo
not-operator this act is the operator's, and you are bo
[exit 2]

$ dinah unblock rel-2
rel-2  Draft the changelog  [Intake / ready]
[exit 0]
```

Release a card as soon as you stop working it, and the queue stays honest about
what is available. A claim can also carry its own expiry. When one lapses, the
card returns to the queue with the lapse recorded:

```
$ dinah release rel-1
rel-1  Write the release notes  [Doing / ready]
[exit 0]

$ dinah claim rel-1 --expires 8h --quiet
rel-1  Write the release notes  [Doing / active]
  held by ana
[exit 0]

$ dinah move rel-1 done
rel-1  Write the release notes  [Done / active]
  held by ana

Instructions, this workbench:
Every card on this workbench ends with a line in the changelog.

Moves this card may make:
  1d2f2a07a38b  Intake                          backward
  df8cd3d7f024  Doing                           backward
[exit 0]
```

A card may always move backward out of a done state, but a forward move out of
one is refused with `terminal`. Both remaining moves above are backward for that
reason.

## Everything below a card

A card is a directory, its comments and attachments are directories inside it,
and giving `attach` a file copies the bytes in under their own name:

```
$ dinah attach rel-1 notes.txt
rel-1  Write the release notes  [Done / active]
  held by ana
[exit 0]
```

Pass `--replace` to swap the bytes of an attachment that is already there.

Anything below a card is addressable by a path reference: the card's reference
followed by slash-separated segments. The segments are `comments`,
`attachments`, `checklist`, `journal`, and `card`, plus `oq`, `ac`, and `d` as
shorthands for the three checklist kinds. Reaching past an attachment into
`payload` gets you the file itself. No command in this version files a checklist
item, though, so the checklist segments address items that something else has
already written.

A collection takes either a twelve-hex identifier or a one-based position, and a
position counts in the order the things were written. Every comment, attachment,
and checklist item carries a creation ordinal of its own. That is the `ordinal`
line in the frontmatter below, and it is what keeps `rel-1/comments/1` the
comment you wrote first as others arrive.

```
$ dinah show rel-1/attachments/1
---
filename: notes.txt
provenance: ana
ordinal: 1
---
[exit 0]

$ dinah show rel-1/comments/1
---
ts: 2026-08-17T09:41:54Z
author: ana
ordinal: 1
---
Drafted entries for the four merged branches.
[exit 0]

$ dinah show rel-1/comments/2
---
ts: 2026-08-17T09:41:56Z
author: ana
ordinal: 2
---
Second half needs the signing certificate.
[exit 0]

$ dinah path rel-1/attachments/1/payload
C:\work\release-notes\cards\55cce32b3c43\attachments\d1af8cf49f49\payload\notes.txt
[exit 0]
```

An older workbench, written before ordinals existed, has entities carrying none,
and there a position is only as good as the directory listing. The repair is
`dinah check --migrate-ordinals`, and the section on defects below runs it.

The full record of a card is its journal, and `log` renders it oldest first:

```
$ dinah path rel-1
C:\work\release-notes\cards\55cce32b3c43\card.md
[exit 0]

$ dinah path rel-1/journal
C:\work\release-notes\cards\55cce32b3c43\journal.ndjson
[exit 0]

$ dinah log rel-1
  2026-08-17T09:41:49Z  created       ana             Write the release notes
  2026-08-17T09:41:53Z  claimed       ana
  2026-08-17T09:41:54Z  moved         ana             Intake to Doing (override)
  2026-08-17T09:41:54Z  commented     ana
  2026-08-17T09:41:56Z  commented     ana
  2026-08-17T09:41:56Z  released      ana
  2026-08-17T09:41:56Z  claimed       ana
  2026-08-17T09:41:56Z  moved         ana             Doing to Done
  2026-08-17T09:41:56Z  attached      ana
[exit 0]
```

The override shows up on the move that used it. Each act names the states it
refers to as they were titled at the time. Rename a state later and the history
still reads as it did.

## Taking things out

`archive` takes a card, or a comment or attachment on one, out of the listings
and keeps its files, but `delete` destroys the same things and their history.
Both are quiet on success.

```
$ dinah archive rel-3
[exit 0]

$ dinah ls done
  rel-1         active    Write the release notes
[exit 0]

$ dinah delete rel-1/comments/2
dinah.unconfirmed delete destroys history, so it needs --yes
[exit 2]

$ dinah delete rel-1/comments/2 --yes
[exit 0]
```

Archived cards move under `archive/cards/` in the workbench and stop appearing
in listings. Deletion is not recoverable. `delete` requires `--yes` for that
reason.

## When the files are wrong

Hand-editing is legal, but it makes mistakes possible, and `check` is how you
find them. Here a card names a state that no longer exists:

```
$ dinah check
  a card names state 000000000000, which this workbench does not declare (C:\work\release-notes\cards\eafed105f127\card.md)
  the journal puts this card in state 1d2f2a07a38b, and its frontmatter disagrees (C:\work\release-notes\cards\eafed105f127\card.md)
2 defects.
[exit 2]
```

Every line names the file to open. Fix it with an editor and run `check` again:

```
$ dinah check
No structural defects found.
[exit 0]
```

`check` also catches a claim without the substate that implies it, a block with
no reason, a link pointing at no card, a journal whose last line was cut off,
and a directory sitting where an entity should be. It reads and reports; it
changes nothing unless you ask it to.

Two of the things it reports are the marks of an older workbench rather than a
mistake, and each has a flag that repairs it. A state written before slugs
existed carries none, and an entity written before ordinals existed carries
none:

```
$ dinah check
  entity 263a62237a3b carries no creation ordinal, so its position depends on the directory listing (C:\work\legacy\cards\55cce32b3c43\comments\263a62237a3b\comment.md)
  entity d1af8cf49f49 carries no creation ordinal, so its position depends on the directory listing (C:\work\legacy\cards\55cce32b3c43\attachments\d1af8cf49f49\attachment.md)
  state 1d2f2a07a38b carries no slug, so it is reachable only by its identifier or its quoted title (C:\work\legacy\states\1d2f2a07a38b\state.md)
  state df8cd3d7f024 carries no slug, so it is reachable only by its identifier or its quoted title (C:\work\legacy\states\df8cd3d7f024\state.md)
  state acd3a55081b7 carries no slug, so it is reachable only by its identifier or its quoted title (C:\work\legacy\states\acd3a55081b7\state.md)
5 defects.
[exit 2]

$ dinah check --migrate-slugs
Assigned 3 state slugs.
  intake                  Intake
  doing                   Doing
  done                    Done
  entity 263a62237a3b carries no creation ordinal, so its position depends on the directory listing (C:\work\legacy\cards\55cce32b3c43\comments\263a62237a3b\comment.md)
  entity d1af8cf49f49 carries no creation ordinal, so its position depends on the directory listing (C:\work\legacy\cards\55cce32b3c43\attachments\d1af8cf49f49\attachment.md)
2 defects.
[exit 2]

$ dinah check --migrate-ordinals
Stamped 2 creation ordinals.
No structural defects found.
[exit 0]
```

Each migration prints what it wrote and then reports whatever it did not fix, so
the exit code stays 2 until the workbench is clean. Slugs come from the titles,
and ordinals are read back out of each card's journal, so the journal is worth
keeping intact. Run either one against a copy first if the workbench
matters to you.

A third flag, `--finish`, is for a rarer case. A structural act interrupted part
way through, by a power cut or a killed process, leaves a lock file behind
naming what it was doing. `check` reports that as an interrupted act, and
`check --finish` reads the journal to decide whether the act reached its point
of record, then completes it or rolls it back. Run `check` after any hand edit,
and before you commit a workbench to version control.

## Driving Dinah from a script

Four exit codes cover every outcome. A script should keep them apart, because
each one calls for something different.

| Code | Outcome | What to do |
| --- | --- | --- |
| 0 | ok | It happened. |
| 2 | refused | A rule said no. The refusal name says which. |
| 3 | stale | The card moved between your reading it and your acting. Read it again and retry. |
| 4 | unreachable | The question could not be asked at all. |

Pass `--json` for the machine-readable form. Under it a refusal writes its name
to standard output as a field, and that is the portable way to find out which
rule said no. Standard output alone:

```
$ dinah claim rel-9 --json
{
  "outcome": "refused",
  "verb": "claim",
  "refusal": "unknown-card",
  "detail": "rel-9",
  "affordances": [
    "status",
    "states",
    "ls",
    "next"
  ]
}
[exit 2]
```

Standard error still carries the sentence a person reads, whether or not
`--json` was asked for, and without the flag that line is all you get, with the
refusal name as the first word on it:

```
$ dinah claim rel-9
unknown-card this workbench carries no card rel-9
[exit 2]
```

The sentence after the name is for a person and may be translated. The name
never is. In bash or zsh, that first word comes out with `cut`:

```
$ dinah claim rel-9 2>&1 >/dev/null | cut -d' ' -f1
unknown-card
```

That block is POSIX-only. The exit code you branch on is Dinah's own, so capture
it before a pipe puts another program's status in its place. PowerShell also
decorates a native command's error stream when it redirects one, so the `--json`
route above is the one that behaves the same everywhere.

Some refusal names come from the shared rules that every Dinah-compatible tool
follows; others are this tool's own. The tool's own names carry a `dinah.`
prefix. Matching on that prefix is how you tell the two groups apart.
`unknown-card` and `at-capacity` are shared. `dinah.unknown-key` and
`dinah.unconfirmed` are this tool's.

Setting `DINAH_FORMAT=json` gets the machine-readable form from every call
without the flag, and setting an environment variable is the first of the two
things that differ between shells.

In bash or zsh:

```
export DINAH_FORMAT=json
```

In PowerShell:

```
$env:DINAH_FORMAT = 'json'
```

The JSON always uses the machine spellings, such as `ready` and `unknown-card`,
and never translates them, so the same command emits the same bytes under any
language setting.

```
$ dinah ls intake --json
{
  "state": "1d2f2a07a38b",
  "cards": [
    {
      "id": "eafed105f127",
      "ref": "rel-2",
      "title": "Draft the changelog",
      "state": "1d2f2a07a38b",
      "state_title": "Intake",
      "substate": "ready",
      "revision": "sha256:f444a5cd6bc00b33da0e8db29e6b8f40f9e8b7ca200b9129dc35161376e2e772"
    }
  ]
}
[exit 0]
```

The `revision` is the card's content as you read it, reduced to a hash. The
stale outcome is measured against it, so a tool that reads a card, thinks, and
then acts has a way to find out that the card moved in between.

`path` is built to be composed with. It writes one absolute path to standard
output and nothing else, and substituting that into another command is the
second of the two things that differ between shells.

In bash or zsh:

```
code "$(dinah path rel-1/comments/1)"
```

In PowerShell:

```
code (dinah path rel-1/comments/1)
```

## Read the workbench in your own language

Dinah works out the display language by trying the `--lang` flag first, then
`DINAH_LANG`, then your user config, then the operating system locale, and
English if none of them answers. The locale sits below the config because it
describes the machine rather than the person reading the screen.

```
$ dinah --lang hi status
Release 0.2  (C:\work\release-notes)
ana के रूप में, संचालक: हाँ

  1d2f2a07a38b  intake                  Intake                          आवक       1
  df8cd3d7f024  doing                   Doing                           काम       0/1
  acd3a55081b7  done                    Done                            समाप्त    1

आपके पास:
  rel-1         Write the release notes
[exit 0]
```

Titles you wrote stay as you wrote them, since they are your text rather than
the tool's, and so do the slugs derived from them. To see which languages this
build actually carries, ask:

```
$ dinah version --catalogs
dinah 0.1.0
conforms to dinah-core/1.0
storage format 1

Catalogs:
  en      222/222
  af      0/222
  cs      0/222
  de      0/222
  es      0/222
  fil     0/222
  hi      222/222
  id      0/222
[exit 0]
```

A catalog with no entries falls back to English message by message, so asking
for one of those changes nothing yet.

## Open a card in your editor

`edit` accepts the same references `path` does and hands the file to your
editor, looking at `DINAH_EDITOR` first, then your user config, then `VISUAL`,
then `EDITOR`, and falling back to a platform default. The Dinah-specific
variable sits on top because `EDITOR` is shared with every other tool you run,
and somebody who wants git and Dinah opening different editors has nowhere else
to say so.

Set `DINAH_EDITOR` the way your shell sets any variable, as shown in the
scripting section above. The run below points it at `more` so the block has
something to print instead of opening a window:

```
$ dinah edit rel-1
---
title: Write the release notes
number: 1
state: acd3a55081b7
substate: active
claim_holder: ana
claim_since: 2026-08-17T09:41:56Z
claim_expires: 2026-08-17T17:41:56Z
---
[exit 0]
```

The value is run as a program name, so give it the name of an editor rather than
a command line with flags in it:

```
$ dinah config set editor notepad
[exit 0]

$ dinah config get editor
notepad
[exit 0]
```

If none of those is set and no fallback editor is present, `edit` refuses with
`dinah.no-editor`.

## Work on a workbench you are not standing in

Dinah walks up from the working directory to find its workbench, so standing
anywhere inside one is enough. To see what that walk reaches from where you are
now, ask `workbenches`. It answers with rows rather than a refusal, and no rows
is as honest an answer as several:

```
$ dinah workbenches
  Release 0.2                     rel             C:\work\release-notes
[exit 0]

$ cd ..

$ dinah workbenches
no workbench is reachable from here
[exit 0]
```

The walk climbs, so a workbench in a directory beside you is out of reach and a
workbench above you is not. From outside, name the one you want with
`--workbench`, or set `DINAH_WORKBENCH`:

```
$ dinah --workbench release-notes status
Release 0.2  (C:\work\release-notes)
acting as ana, operator: yes

  1d2f2a07a38b  intake                  Intake                          intake    1
  df8cd3d7f024  doing                   Doing                           work      0/1
  acd3a55081b7  done                    Done                            done      1

You are holding:
  rel-1         Write the release notes
[exit 0]
```

Your home directory can hold workbenches of its own, in the `.dinah` directory
beside your settings, and those are what the walk falls back to when the climb
finds nothing. Two or more of them is a question the tool will not answer for
you:

```
$ dinah workbenches
  0f1e2d3c4b5a                    bet             C:\Users\ana\.dinah\0f1e2d3c4b5a
  a1b2c3d4e5f6                    alp             C:\Users\ana\.dinah\a1b2c3d4e5f6
[exit 0]

$ dinah status
dinah.ambiguous-bench 0f1e2d3c4b5a (C:\Users\ana\.dinah\0f1e2d3c4b5a); a1b2c3d4e5f6 (C:\Users\ana\.dinah\a1b2c3d4e5f6) are all reachable from C:\Users\ana\.dinah; choose one with --workbench <path>, or run from inside it
[exit 2]
```

A bare `show` prints that same listing. You asked to be shown something, the
tool cannot tell which workbench you meant, and the choices are what it has to
show. When nothing is reachable at all, the refusal is `dinah.no-bench-found`,
and it names both the directory the climb started from and the home directory it
fell back to.

`DINAH_HOME` moves that fallback directory. Point it somewhere else and your
settings file and any workbenches under it come from there instead. That is how
you work against a scratch setup without touching your own.

## Hand a workbench to somebody else

`export` prints the whole workbench definition in the shared exchange format.
Another program built to the same rules reads that form:

```
$ cd release-notes

$ dinah export
{
  "profile": "dinah-core/1.0",
  "states": [
    {
      "id": "1d2f2a07a38b",
      "kind": "intake",
      "slug": "intake",
      "title": "Intake"
    },
    {
      "capacity": 1,
      "id": "df8cd3d7f024",
      "instructions": "Work the card until it is finished or until something stops you.\nLeave a comment saying what you did before you carry it on.\n",
      "kind": "work",
      "slug": "doing",
      "title": "Doing"
    },
    {
      "id": "acd3a55081b7",
      "kind": "done",
      "slug": "done",
      "title": "Done"
    }
  ],
  "title": "Release 0.2"
}
[exit 0]
```

`extract` writes the same definition to a directory as a reusable template,
carrying the flow and the instructions and none of the cards. `init --from`
starts a new workbench from one:

```
$ dinah extract ../release-template
Definition written to ../release-template.
[exit 0]

$ cd ..
$ mkdir release-0.3
$ cd release-0.3

$ dinah init --from ../release-template --slug rel3 --operator ana
Workbench created at C:\work\release-0.3.
[exit 0]

$ dinah states
  1d2f2a07a38b  intake                  Intake                          intake    0
  df8cd3d7f024  doing                   Doing                           work      0/1
  acd3a55081b7  done                    Done                            done      0
[exit 0]
```

```
$ cd ../release-notes
```

The template carries the state identifiers and the slugs too, so the new
workbench and the old one answer to the same ones. That last `cd` puts you back
in the workbench this guide started in. The commands below expect to run there.

## The guides that ship in the binary

The binary carries its own guides, so they are readable with no network and no
repository checkout:

```
$ dinah guide
  getting-started     Getting started
  verbs               The five verbs
  workbench-layout    What a workbench looks like on disk
[exit 0]
```

`dinah guide workbench-layout` is the one to read before you start editing files
by hand, since it maps the whole directory.

## Point an agent at the workbench

`dinah mcp` serves the workbench over MCP on its standard input and output, so
an AI colleague can work the same board you do. Configure it in your MCP client
as the command `dinah mcp`, with the workbench directory as the working
directory or `DINAH_WORKBENCH` set to it. The server hands the client the rules
for working this workbench and one tool for each command, so the agent claims,
moves, releases, and blocks under the same rules and leaves the same journal
entries you do.

Give the agent an actor name of its own through `DINAH_ACTOR` so the record
shows who did what.

## Where to look next

`dinah help <command>` lists a command's arguments and every reason it can
refuse, in the order the checks are made. That ordering is worth knowing when
you are working out which of two possible refusals you are looking at.

```
$ dinah help claim
claim <card> [--expires <duration>]

take up a ready card

Refusals, in the order the checks are made:
  1  the workbench declares a major number the tool implements unsupported-version
  2  the workbench designates an operator                no-operator
  3  the card exists                                     unknown-card
  4  the request names an owner                          no-owner
  5  the owner named as holder is the owner asking       not-requester
  6  the card's substate is not `blocked`                blocked
  7  the card's substate is not `active`                 held

Exit codes: 0 ok, 2 refused, 3 stale, 4 unreachable.
[exit 0]
```
