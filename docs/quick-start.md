# Dinah quick start

This guide walks one workbench, the folder of plain files that holds a board's
work, from an empty directory to a finished card, and it touches every command
the binary offers along the way. Every transcript below is real output from
`dinah 0.1.0`. The blocks that show a file's contents are what you type into
that file, and they are marked as such where they appear.

Read it in order the first time. After that, the section headings are the index.

## Shells

The command lines below run unchanged in bash, zsh and PowerShell, because they
use only plain arguments, relative paths, `mkdir` and `cd`, and let Dinah find
the workbench by walking up from the working directory. Two things genuinely
differ between those shells: setting an environment variable, and substituting
one command's output into another. Each appears once below as a labeled pair,
and one block uses a POSIX-only utility and is labeled where it appears.

The leading `$` marks a command line and is not part of the command. This
session ran on Windows, so the paths in the output are Windows paths; on macOS
or Linux the same runs print POSIX paths.

## What the binary tells you about itself

```
$ dinah version
dinah 0.1.0
conforms to dinah-core/1.0
storage format 1
[exit 0]
```

Those lines carry three different facts. The first is the build. The second
names the shared rule set this build follows, and another tool built to the same
rules can read the same workbench and reach the same answers about it. The third
names the on-disk format.

`dinah help` lists every command. The flag spelling you may reach for first is
not one the binary accepts:

```
$ dinah --help
dinah.usage --help was not understood; run dinah help for the surface
[exit 2]
```

That refusal calls the list of commands the surface, and the list is all the
word means. Running `dinah` with no arguments prints the same list. For one
command's arguments and the reasons it can say no, ask for it by name with
`dinah help move`.

## Open a workbench

A workbench is a directory of plain-text files. Create one where the work is,
and put it under version control alongside the project it belongs to.

```
$ mkdir release-notes
$ cd release-notes

$ dinah init --slug rel --operator ana
Bench created at C:\work\release-notes.
[exit 0]
```

Dinah's own messages say bench where this guide says workbench, and the two
words mean the same thing here.

The slug is the prefix every card reference carries, so the first card you file
here will be `rel-1`. Leave `--slug` out and Dinah derives one from the
directory name. The operator is the owner who answers for the workbench, and
only the operator can lift a block or force a move past a limit, so a workbench
with nobody in that seat has acts nobody can perform. Leave `--operator` out
and Dinah records whoever you are acting as.

That call wrote `workbench.md` and a `states/` directory holding one file per
state. Nothing else exists yet.

Every command from here on runs inside the workbench directory. Dinah finds the
workbench by walking up from wherever you are, the way git finds a repository,
so none of them needs a path.

## Say who you are

Every act Dinah records carries an owner, and the tool will not invent one.

```
$ dinah whoami
no-owner no owner was resolvable; set one with --actor, DINAH_ACTOR or config actor
[exit 2]
```

The refusal names the three places it looked, in the order it looked. The
`--actor` flag wins, then the `DINAH_ACTOR` environment variable, then your
user config. Set the last of those three once and forget it:

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

`whoami` answers two questions at once, because the second one governs what you
are allowed to do. The settings live in `config.md` under `.dinah` in your home
directory, and they are yours rather than the workbench's, so they follow you to
every workbench you work. The keys this version knows are `actor`, `lang` and
`editor`:

```
$ dinah config set colour green
dinah.unknown-key this tool knows no setting called colour
[exit 2]
```

Dinah preserves keys it does not recognise when it writes the file, so anything
you added there by hand stays.

## Look at the flow

```
$ dinah states
  b4d597d4f7bb  Intake                          intake    0
  ff2602398079  Doing                           work      0
  17d700d9f84b  Done                            done      0
[exit 0]
```

Each row gives a state's identifier, its title, its kind and how many cards
stand in it. The kinds are `intake`, `work` and `done`, and the flow runs in the
order `workbench.md` lists them, so a move to a later state is forward and a
move to an earlier one is backward. Identifiers are generated per workbench, so
yours will differ from the ones printed here.

Run `status` when you sit down. It prints that same list and adds what you
personally hold.

```
$ dinah status
release-notes  (C:\work\release-notes)
acting as ana, operator: yes

  b4d597d4f7bb  Intake                          intake    0
  ff2602398079  Doing                           work      0
  17d700d9f84b  Done                            done      0
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
  - b4d597d4f7bb
  - ff2602398079
  - 17d700d9f84b
---
Every card on this bench ends with a line in the changelog.
```

Then open the `state.md` of one state and do the same. A `wip_limit` caps how
many cards that state will hold:

```
---
wip_limit: 1
title: Doing
kind: work
---
Work the card until it is finished or until something stops you.
Leave a comment saying what you did before you carry it on.
```

Nothing copies that prose anywhere. It is served from where you wrote it, so an
edit reaches every reader at once. Check your work whenever you have been
editing by hand:

```
$ dinah fsck
No structural defects found.
[exit 0]

$ dinah status
Release 0.2  (C:\work\release-notes)
acting as ana, operator: yes

  b4d597d4f7bb  Intake                          intake    0
  ff2602398079  Doing                           work      0/1
  17d700d9f84b  Done                            done      0
[exit 0]
```

The limit shows up as `0/1` in the count column. To read what a position serves
without touching a card, ask for it:

```
$ dinah instructions doing

Instructions, this bench:
Every card on this bench ends with a line in the changelog.

Instructions, this state:
Work the card until it is finished or until something stops you.
Leave a comment saying what you did before you carry it on.
[exit 0]
```

States can be named by title as well as by identifier, and the match ignores
case, so `doing` finds the state titled `Doing`. `dinah instructions` also takes
a card reference, in which case it serves the position that card is standing at.

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
and `show` prints one card. Listings run in queue order, oldest arrival first.
Arrival is stamped to the second, so cards filed inside the same second tie, and
the tie falls to their random identifiers. Space your own `add` calls out if you
want the order below.

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

Five commands change where a card stands: `claim`, `move`, `release`, `block`
and `unblock`. Dinah's own guide calls these five the verbs, and you can read
that guide with `dinah guide verbs`. The shared rules fix what each one does,
so a second tool reading the same workbench answers the same way.

Work is taken rather than handed out, so you claim your own card:

```
$ dinah claim rel-1
rel-1  Write the release notes  [Intake / active]
  held by ana

Instructions, this bench:
Every card on this bench ends with a line in the changelog.

Moves this card may make:
  ff2602398079  Doing                           forward
  17d700d9f84b  Done                            forward
[exit 0]
```

A successful claim serves you the instructions of the position and the moves the
flow allows from it, so you do not have to remember either. Pass `--quiet` when
you have read them already.

A held card is not available to anybody else:

```
$ dinah claim rel-1 --actor bo
held ana holds this card
[exit 2]
```

`move` carries a card to another state and changes nothing else, so a holder who
moves a card still holds it. `Doing` is capped at one and already has `rel-3` in
it, so the move is refused:

```
$ dinah move rel-1 doing
at-capacity state ff2602398079 has reached its limit
[exit 2]
```

The override is the operator's alone, and it is recorded on the move:

```
$ dinah move rel-1 doing --override
rel-1  Write the release notes  [Doing / active]
  held by ana

Instructions, this bench:
Every card on this bench ends with a line in the changelog.

Instructions, this state:
Work the card until it is finished or until something stops you.
Leave a comment saying what you did before you carry it on.

Moves this card may make:
  b4d597d4f7bb  Intake                          backward
  17d700d9f84b  Done                            forward
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

Pass a single dash instead of the text and `comment` reads standard input, which
is how a script hands it something longer than a command line wants.

When something stops the work, say so on the card. A block frees the card and
records why. That is what turns an obstacle into something visible:

```
$ dinah block rel-2 "Waiting on the signing certificate" --kind external
rel-2  Draft the changelog  [Intake / blocked]
  blocked: Waiting on the signing certificate
[exit 0]
```

The reason is required and it is prose, because the things that stop real work
vary. The `--kind` is a short label of your own choosing for grouping later.
Blocks show up in `status`:

```
$ dinah status
Release 0.2  (C:\work\release-notes)
acting as ana, operator: yes

  b4d597d4f7bb  Intake                          intake    1
  ff2602398079  Doing                           work      2/1
  17d700d9f84b  Done                            done      0

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

Release a card as soon as you stop working it, so the queue stays honest about
what is available. A claim can also carry its own expiry, and a lapsed claim
returns the card to the queue with the lapse recorded:

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

Instructions, this bench:
Every card on this bench ends with a line in the changelog.

Moves this card may make:
  b4d597d4f7bb  Intake                          backward
  ff2602398079  Doing                           backward
[exit 0]
```

A card may always move backward out of a done state, and a forward move out of
one is refused with `terminal`. Both remaining moves above are backward for that
reason.

## Everything below a card

A card is a directory, and its comments and attachments are directories inside
it. Give `attach` a file and it copies the bytes in under their own name:

```
$ dinah attach rel-1 notes.txt
rel-1  Write the release notes  [Done / active]
  held by ana
[exit 0]
```

Pass `--replace` to swap the bytes of an attachment that is already there.

Anything below a card is addressable by a path reference. A path reference is
the card's reference followed by slash-separated segments. The segments are
`comments`, `attachments`, `checklist`, `journal` and `card`, plus `oq`, `ac`
and `d` as shorthands for the three checklist kinds. Reaching past an
attachment into `payload` gets you the file itself. No command in this version
files a checklist item, so the checklist segments address items that something
else has already written.

A collection takes either a twelve-hex identifier or a one-based position. The
position selects from the collection in identifier order, and identifiers are
random, so `rel-1/comments/1` is not reliably the comment you wrote first.
Positions are convenient at a terminal where you can see what came back. Use the
identifier when a script has to name a particular comment or attachment.

```
$ dinah show rel-1/attachments/1
---
filename: notes.txt
provenance: ana
---
[exit 0]

$ dinah show rel-1/comments/1
---
ts: 2026-08-17T01:50:32Z
author: ana
---
Drafted entries for the four merged branches.
[exit 0]

$ dinah show rel-1/comments/2
---
ts: 2026-08-17T01:50:34Z
author: ana
---
Second half needs the signing certificate.
[exit 0]

$ dinah path rel-1/attachments/1/payload
C:\work\release-notes\cards\878aa3d95917\attachments\ddac956ad996\payload\notes.txt
[exit 0]
```

Those two comments came back in the order they were written, and a second run of
the same session returned them the other way round. That is the identifier
ordering above, and it is why the sentence beside a position has to be careful.

The full record of a card is its journal, and `log` renders it oldest first:

```
$ dinah path rel-1
C:\work\release-notes\cards\878aa3d95917\card.md
[exit 0]

$ dinah path rel-1/journal
C:\work\release-notes\cards\878aa3d95917\journal.ndjson
[exit 0]

$ dinah log rel-1
  2026-08-17T01:50:27Z  created       ana             Write the release notes
  2026-08-17T01:50:32Z  claimed       ana
  2026-08-17T01:50:32Z  moved         ana             Intake to Doing (override)
  2026-08-17T01:50:32Z  commented     ana
  2026-08-17T01:50:34Z  commented     ana
  2026-08-17T01:50:34Z  released      ana
  2026-08-17T01:50:34Z  claimed       ana
  2026-08-17T01:50:34Z  moved         ana             Doing to Done
  2026-08-17T01:50:35Z  attached      ana
[exit 0]
```

The override shows up on the move that used it. Each act names the states it
refers to as they were titled at the time, so renaming a state later does not
change what the history says.

## Taking things out

`archive` takes a card, or a comment or attachment on one, out of the live set
and keeps its files. The live set is what listings show you. `delete` destroys
the same things and their history instead. Both are quiet on success.

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
in listings. Deletion is not recoverable, so `delete` requires `--yes`.

## When the files are wrong

Hand-editing is legal, so mistakes are possible, and `fsck` is how you find
them. Here a card names a state that no longer exists:

```
$ dinah fsck
  a card names state 000000000000, which this bench does not declare (C:\work\release-notes\cards\12eaa9218650\card.md)
  the journal puts this card in state b4d597d4f7bb, and its frontmatter disagrees (C:\work\release-notes\cards\12eaa9218650\card.md)
2 defects.
[exit 2]
```

Every line names the file to open. Fix it with an editor and run `fsck` again:

```
$ dinah fsck
No structural defects found.
[exit 0]
```

`fsck` also catches a claim without the substate that implies it, a block with
no reason, and a link pointing at no card. Run it after any hand edit, and
before you commit a workbench to version control.

## Driving Dinah from a script

Four exit codes cover every outcome, and a script should keep them apart because
each calls for something different.

| Code | Outcome | What to do |
| --- | --- | --- |
| 0 | ok | It happened. |
| 2 | refused | A rule said no. The refusal name says which. |
| 3 | stale | The card moved between your reading it and your acting. Read it again and retry. |
| 4 | unreachable | The question could not be asked at all. |

Pass `--json` for the machine-readable form. Under it a refusal writes its name
to standard output as a field. That is the portable way to find out which rule
said no. Standard output alone:

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
`--json` was asked for. Without the flag that line is all you get, and the
refusal name is the first word on it:

```
$ dinah claim rel-9
unknown-card this bench carries no card rel-9
[exit 2]
```

The sentence after the name is for a person and may be translated. The name
never is. In bash or zsh, that first word comes out with `cut`:

```
$ dinah claim rel-9 2>&1 >/dev/null | cut -d' ' -f1
unknown-card
```

That block is POSIX-only, and the exit code you branch on is Dinah's own, so
capture it before a pipe puts another program's status in its place. PowerShell
decorates a native command's error stream when it redirects one, so the `--json`
route above is the one that behaves the same everywhere.

Some refusal names come from the shared rules that every Dinah-compatible tool
follows, and some are this tool's own. The tool's own names carry a `dinah.`
prefix, and matching on that prefix is how you tell the two groups apart.
`unknown-card` and `at-capacity` are shared. `dinah.unknown-key` and
`dinah.unconfirmed` are this tool's.

Setting `DINAH_FORMAT=json` gets the machine-readable form from every call
without the flag. Setting an environment variable is the first of the two
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
  "state": "b4d597d4f7bb",
  "cards": [
    {
      "id": "12eaa9218650",
      "ref": "rel-2",
      "title": "Draft the changelog",
      "state": "b4d597d4f7bb",
      "state_title": "Intake",
      "substate": "ready",
      "revision": "sha256:9cc4b64dd9a307cf6ada7d960d59aac80150bd63fc19800af8dbf2458a6d6131"
    }
  ]
}
[exit 0]
```

The `revision` is the card's content as you read it, reduced to a hash. It is
what the stale outcome is measured against, so a tool that reads a card, thinks,
and then acts has a way to find out that the card moved in between.

`path` is built to be composed with. It writes one absolute path to standard
output and nothing else. Substituting that into another command is the second of
the two things that differ between shells.

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

  b4d597d4f7bb  Intake                          आवक       1
  ff2602398079  Doing                           काम       0/1
  17d700d9f84b  Done                            समाप्त    1

आपके पास:
  rel-1         Write the release notes
[exit 0]
```

Titles you wrote stay as you wrote them, since they are your text rather than
the tool's. To see which languages this build actually carries, ask:

```
$ dinah version --catalogs
dinah 0.1.0
conforms to dinah-core/1.0
storage format 1

Catalogs:
  en      187/187
  af      0/187
  cs      0/187
  de      0/187
  es      0/187
  fil     0/187
  hi      187/187
  id      0/187
[exit 0]
```

A catalog with no entries falls back to English message by message, so asking
for one of those changes nothing yet.

## Open a card in your editor

`edit` accepts the same references `path` does and hands the file to your
editor. It looks at `DINAH_EDITOR` first, then your user config, then `VISUAL`,
then `EDITOR`, and falls back to a platform default. The Dinah-specific variable
sits on top because `EDITOR` is shared with every other tool you run, and
somebody who wants git and Dinah opening different editors has nowhere else to
say so.

Set `DINAH_EDITOR` the way your shell sets any variable, as shown in the
scripting section above. The run below points it at `more` so the block has
something to print instead of opening a window:

```
$ dinah edit rel-1
---
title: Write the release notes
number: 1
state: 17d700d9f84b
substate: active
claim_holder: ana
claim_since: 2026-08-17T01:50:34Z
claim_expires: 2026-08-17T09:50:34Z
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
anywhere inside one is enough. From outside, name the workbench with `--bench`,
or set `DINAH_BENCH`:

```
$ cd ..

$ dinah --bench release-notes status
Release 0.2  (C:\work\release-notes)
acting as ana, operator: yes

  b4d597d4f7bb  Intake                          intake    1
  ff2602398079  Doing                           work      0/1
  17d700d9f84b  Done                            done      1

You are holding:
  rel-1         Write the release notes
[exit 0]
```

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
      "id": "b4d597d4f7bb",
      "kind": "intake",
      "title": "Intake"
    },
    {
      "capacity": 1,
      "id": "ff2602398079",
      "instructions": "Work the card until it is finished or until something stops you.\nLeave a comment saying what you did before you carry it on.\n",
      "kind": "work",
      "title": "Doing"
    },
    {
      "id": "17d700d9f84b",
      "kind": "done",
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
Bench created at C:\work\release-0.3.
[exit 0]

$ dinah states
  b4d597d4f7bb  Intake                          intake    0
  ff2602398079  Doing                           work      0/1
  17d700d9f84b  Done                            done      0
[exit 0]
```

```
$ cd ../release-notes
```

The template carries the state identifiers too, so the new workbench and the
old one answer to the same ones. That last `cd` puts you back in the workbench
this guide started in. The commands below expect to run there.

## The guides that ship in the binary

The binary carries its own guides, so they are readable with no network and no
repository checkout:

```
$ dinah guide
  bench-layout        What a bench looks like on disk
  getting-started     Getting started
  verbs               The five verbs
[exit 0]
```

`dinah guide bench-layout` is the one to read before you start editing files by
hand, since it maps the whole directory.

## Point an agent at the workbench

`dinah mcp` serves the workbench over MCP on its standard input and output, so
an AI colleague can work the same board you do. Configure it in your MCP client
as the command `dinah mcp`, with the workbench directory as the working
directory or `DINAH_BENCH` set to it. The server hands the client the rules for
working this workbench and one tool for each command, so the agent claims,
moves, releases and blocks under the same rules and leaves the same journal
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
