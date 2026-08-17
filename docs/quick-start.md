# Dinah quick start

This guide walks one bench from an empty directory to a finished card, and it
touches every command the binary offers along the way. Every block below is real
output from `dinah 0.1.0`.

Read it in order the first time. After that, the section headings are the index.

## What the binary tells you about itself

```
$ dinah version
dinah 0.1.0
conforms to dinah-core/1.0
storage format 1
[exit 0]
```

Those lines carry three different facts. The first is the build. The second is
the coordination contract this build implements, and it is what lets another
tool read the same bench and agree with you about it. The third names the
on-disk format.

The command that prints the surface is `dinah help`. The flag spelling you may
reach for first is not one the binary accepts:

```
$ dinah --help
dinah.usage --help was not understood; run dinah help for the surface
[exit 2]
```

Running `dinah` with no arguments prints the same surface as `dinah help`, so
either gets you the list of commands. For one command's arguments and the
reasons it can say no, ask for it by name with `dinah help move`.

## Make a bench

A bench is a directory of plain-text files. Create one where the work is, and
put it under version control alongside the project it belongs to.

```
$ dinah init --slug rel --operator ana
Bench created at C:\work\release-notes.
[exit 0]
```

The slug is the prefix every card reference carries, so the first card you file
here will be `rel-1`. Leave `--slug` out and Dinah derives one from the
directory name. The operator is the owner who answers for the bench, and only
the operator can lift a block or force a move past a limit, so a bench with
nobody in that seat has acts nobody can perform. Leave `--operator` out and
Dinah records whoever you are acting as.

That call wrote `workbench.md` and a `states/` directory holding one file per
station. Nothing else exists yet.

## Say who you are

Every act Dinah records carries an owner, and the tool will not invent one.

```
$ dinah whoami
no-owner no owner was resolvable; set one with --actor, DINAH_ACTOR or config actor
[exit 2]
```

The refusal names the three places it looked, in the order it looked. The
`--actor` flag wins, then the `DINAH_ACTOR` environment variable, then your
user config. Set the bottom layer once and forget it:

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
directory, and they are yours rather than the bench's, so they follow you to
every bench you work. The keys this version knows are `actor`, `lang` and
`editor`:

```
$ dinah config set colour green
dinah.unknown-key this tool knows no setting called colour
[exit 2]
```

A key already in the file that the tool does not recognise survives a write
untouched, so nothing you put there by hand is lost when Dinah saves.

## Look at the flow

```
$ dinah states
  35d6482a95d1  Intake                          intake    0
  423b4cf9aab7  Doing                           work      0
  870e0ca92e06  Done                            done      0
[exit 0]
```

Each row gives a state's identifier, its title, its kind and how many cards
stand in it. The kinds are `intake`, `work` and `done`, and the flow runs in the
order `workbench.md` lists them, so a move to a later state is forward and a
move to an earlier one is backward.

`status` adds what you personally hold on top of that, which makes it the
command worth running when you sit down.

```
$ dinah status
release-notes  (C:\work\release-notes)
acting as ana, operator: yes

  35d6482a95d1  Intake                          intake    0
  423b4cf9aab7  Doing                           work      0
  870e0ca92e06  Done                            done      0
[exit 0]
```

## Edit the bench by hand

The files are the interface. Open `workbench.md`, give the bench a real title,
and write the standing instructions under the frontmatter:

```
---
format: 1
profile: dinah-core/1.0
title: Release 0.2
slug: rel
operator: ana
states:
  - 35d6482a95d1
  - 423b4cf9aab7
  - 870e0ca92e06
---
Every card on this bench ends with a line in the changelog.
```

Then open `states/423b4cf9aab7/state.md` and do the same for one station. A
`wip_limit` caps how many cards the state will hold:

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

  35d6482a95d1  Intake                          intake    0
  423b4cf9aab7  Doing                           work      0/1
  870e0ca92e06  Done                            done      0
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
it arrives with substate `ready`, which means anybody may pull it. The bracket
after each title is the card's whole position: the state it stands in, then its
substate.

To read the board back, `ls` lists cards and takes an optional state and an
optional `--ready` filter, `next` reports what each state is currently offering,
and `show` prints one card.

```
$ dinah ls
  rel-2         ready     Draft the changelog
  rel-1         ready     Write the release notes
  rel-3         ready     Check the download links
[exit 0]

$ dinah ls doing
  rel-3         ready     Check the download links
[exit 0]

$ dinah ls intake --ready
  rel-2         ready     Draft the changelog
  rel-1         ready     Write the release notes
[exit 0]

$ dinah next
  Intake                          rel-2         Draft the changelog
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

## The five verbs

Five commands change where a card stands: `claim`, `move`, `release`, `block`
and `unblock`. The coordination contract fixes what each one does, so a second
tool reading the same bench answers the same way.

Work is taken rather than handed out, so you claim your own card:

```
$ dinah claim rel-1
rel-1  Write the release notes  [Intake / active]
  held by ana

Instructions, this bench:
Every card on this bench ends with a line in the changelog.

Moves this card may make:
  423b4cf9aab7  Doing                           forward
  870e0ca92e06  Done                            forward
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
at-capacity state 423b4cf9aab7 has reached its limit
[exit 2]
```

A limit that can be walked past silently is not a limit, so the override belongs
to the operator alone and lands in the card's history:

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
  35d6482a95d1  Intake                          backward
  870e0ca92e06  Done                            forward
[exit 0]
```

Say what you did while you are there. `comment` takes the text as an argument,
or reads standard input when the text is a single dash:

```
$ dinah comment rel-1 "Drafted entries for the four merged branches."
rel-1  Write the release notes  [Doing / active]
  held by ana
[exit 0]

$ printf 'Second half needs the certificate.\n' | dinah comment rel-1 -
rel-1  Write the release notes  [Doing / active]
  held by ana
[exit 0]
```

When something stops the work, say so on the card. A block frees the card and
records why, which is what turns an obstacle into something visible:

```
$ dinah block rel-2 "Waiting on the signing certificate" --kind external
rel-2  Draft the changelog  [Intake / blocked]
  blocked: Waiting on the signing certificate
[exit 0]
```

The reason is required and it is prose, because the things that stop real work
vary. The `--kind` is a short label of your own choosing for grouping later.
Blocks surface in `status`:

```
$ dinah status
Release 0.2  (C:\work\release-notes)
acting as ana, operator: yes

  35d6482a95d1  Intake                          intake    1
  423b4cf9aab7  Doing                           work      2/1
  870e0ca92e06  Done                            done      0

You are holding:
  rel-1         Write the release notes

Blocked, waiting on the operator:
  rel-2         Waiting on the signing certificate
[exit 0]
```

Lifting a block is the operator's act, because an obstacle raised is an obstacle
handed to whoever answers for the bench:

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
  35d6482a95d1  Intake                          backward
  423b4cf9aab7  Doing                           backward
[exit 0]
```

Both remaining moves from `Done` are backward. A card may always come back out
of a done state, and the flow refuses to carry one forward out of it.

## Everything below a card

A card is a directory, and comments and attachments are entities inside it. Give
`attach` a file and it copies the bytes in under their own name:

```
$ dinah attach rel-1 notes.txt
rel-1  Write the release notes  [Done / active]
  held by ana
[exit 0]
```

Pass `--replace` to swap the bytes of an attachment that is already there.

Anything below a card is addressable by a path reference, which is the card's
reference followed by slash-separated segments. The segments are `comments`,
`attachments`, `checklist`, `journal` and `card`, plus `oq`, `ac` and `d` as
shorthands for the three checklist kinds. A collection takes either a
twelve-hex identifier or a one-based position, so `rel-1/comments/1` is the
first comment on `rel-1`. Reaching past an attachment into `payload` gets you
the file itself. No command in this version files a checklist item, so the
checklist segments address items that something else has already written.

```
$ dinah show rel-1/attachments/1
---
filename: notes.txt
provenance: ana
---
[exit 0]

$ dinah show rel-1/comments/1
---
ts: 2026-08-17T01:29:15Z
author: ana
---
Drafted entries for the four merged branches.
[exit 0]

$ dinah path rel-1/attachments/1/payload
C:\work\release-notes\cards\bb43cabf0a2b\attachments\4a2edfc85f54\payload\notes.txt
[exit 0]
```

The full record of a card is its journal, and `log` renders it oldest first:

```
$ dinah log rel-1
  2026-08-17T01:29:14Z  created       ana             Write the release notes
  2026-08-17T01:29:15Z  claimed       ana
  2026-08-17T01:29:15Z  moved         ana             Intake to Doing (override)
  2026-08-17T01:29:15Z  commented     ana
  2026-08-17T01:29:15Z  commented     ana
  2026-08-17T01:29:15Z  released      ana
  2026-08-17T01:29:15Z  claimed       ana
  2026-08-17T01:29:15Z  moved         ana             Doing to Done
  2026-08-17T01:29:15Z  attached      ana
[exit 0]
```

The override shows up on the move that used it. Each act names the states it
refers to as they were titled at the time, so the history still reads years
later when those states have been renamed.

## Taking things out

`archive` moves an entity out of the live set and keeps it. `delete` destroys it
and its history. Both are quiet on success.

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

Archived cards move under `archive/cards/` in the bench and stop appearing in
listings. Deleted ones are gone for good, so the confirmation flag is not
optional.

## When the files are wrong

Hand-editing is legal, so mistakes are possible, and `fsck` is how you find
them. Here a card names a state that no longer exists:

```
$ dinah fsck
  a card names state 000000000000, which this bench does not declare (C:\work\release-notes\cards\5d3984fc411b\card.md)
  the journal puts this card in state 35d6482a95d1, and its frontmatter disagrees (C:\work\release-notes\cards\5d3984fc411b\card.md)
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
before you commit a bench to version control.

## Driving Dinah from a script

Four exit codes cover every outcome, and a script should keep them apart because
each calls for something different.

| Code | Outcome | What to do |
| --- | --- | --- |
| 0 | ok | It happened. |
| 2 | refused | A rule said no. The refusal name says which. |
| 3 | stale | The card moved between your reading it and your acting. Read it again and retry. |
| 4 | unreachable | The question could not be asked at all. |

A refusal writes one line to standard error, and the refusal name is the first
whitespace-separated token on that line. The sentence after it is for a person
and may be translated; the name never is.

```
$ dinah claim rel-9
unknown-card this bench carries no card rel-9
[exit 2]

$ dinah claim rel-9 2>&1 >/dev/null | cut -d' ' -f1
unknown-card
```

The exit code you branch on is Dinah's own, so capture it before a pipe puts
another program's status in its place.

Names introduced by this tool rather than by the contract carry a `dinah.`
prefix, which is how you tell the two layers apart when you are matching on
them. `unknown-card` and `at-capacity` come from the contract. `dinah.unknown-key`
and `dinah.unconfirmed` come from the tool.

Pass `--json` for the machine form, or set `DINAH_FORMAT=json` to get it from
every call. The JSON carries canonical tokens and never translates them, so the
same command emits the same bytes under any language setting.

```
$ dinah ls intake --json
{
  "state": "35d6482a95d1",
  "cards": [
    {
      "id": "5d3984fc411b",
      "ref": "rel-2",
      "title": "Draft the changelog",
      "state": "35d6482a95d1",
      "state_title": "Intake",
      "substate": "ready",
      "revision": "sha256:460b757eea0dadd67d37767b66c1cbada56010ba2201bd35f5ecbfc8074997f3"
    }
  ]
}
[exit 0]
```

The `revision` is the card's content as you read it, reduced to a hash. It is
what the stale outcome is measured against, so a tool that reads a card, thinks,
and then acts has a way to find out that the card moved in between.

`path` exists to be composed with. It writes one absolute path to standard
output and nothing else, so command substitution around it is safe in any
shell. In PowerShell:

```
code (dinah path rel-1/comments/1)
```

In a POSIX shell:

```
$EDITOR "$(dinah path rel-1/journal)"
```

## Read the bench in your own language

The display language resolves through the `--lang` flag, then `DINAH_LANG`, then
your user config, then the operating system locale, and English if none of them
answers. The locale sits below the config because it describes the machine
rather than the person reading the screen.

```
$ dinah --lang hi status
Release 0.2  (C:\work\release-notes)
ana के रूप में, संचालक: हाँ

  35d6482a95d1  Intake                          आवक       1
  423b4cf9aab7  Doing                           काम       0/1
  870e0ca92e06  Done                            समाप्त    1

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

`edit` resolves the same references `path` does and hands the file to your
editor. It looks at `DINAH_EDITOR` first, then your user config, then `VISUAL`,
then `EDITOR`, and falls back to a platform default. The Dinah-specific
variable sits on top because `EDITOR` is shared with every other tool you run,
and somebody who wants git and Dinah opening different editors has nowhere else
to say so.

```
$ DINAH_EDITOR=echo dinah edit rel-1
C:\work\release-notes\cards\bb43cabf0a2b\card.md
[exit 0]

$ dinah config set editor "code -w"
[exit 0]

$ dinah config get editor
code -w
[exit 0]
```

If no layer answers and no fallback binary is present, `edit` refuses with
`dinah.no-editor` rather than guessing.

## Work on a bench you are not standing in

Dinah discovers its bench the way git discovers a repository, by walking up from
where you are. Name a different one with `--bench`, or set `DINAH_BENCH`:

```
$ dinah --bench /c/work/release-notes status
Release 0.2  (C:\work\release-notes)
acting as ana, operator: yes

  35d6482a95d1  Intake                          intake    1
  423b4cf9aab7  Doing                           work      0/1
  870e0ca92e06  Done                            done      1

You are holding:
  rel-1         Write the release notes
[exit 0]
```

## Hand a bench to somebody else

`export` writes the bench's interchange form to standard output, which is what
another implementation of the contract reads:

```
$ dinah export
{
  "profile": "dinah-core/1.0",
  "states": [
    {
      "id": "35d6482a95d1",
      "kind": "intake",
      "title": "Intake"
    },
    {
      "capacity": 1,
      "id": "423b4cf9aab7",
      "instructions": "Work the card until it is finished or until something stops you.\nLeave a comment saying what you did before you carry it on.\n",
      "kind": "work",
      "title": "Doing"
    },
    {
      "id": "870e0ca92e06",
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
starts a new bench from one:

```
$ dinah extract /c/work/release-template
Definition written to C:/work/release-template.
[exit 0]

$ dinah init --from /c/work/release-template --slug rel3 --operator ana
Bench created at C:\work\release-0.3.
[exit 0]

$ dinah states
  35d6482a95d1  Intake                          intake    0
  423b4cf9aab7  Doing                           work      0/1
  870e0ca92e06  Done                            done      0
[exit 0]
```

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

## Point an agent at the bench

`dinah mcp` serves the bench over MCP on stdio, which is how an AI colleague
works the same board you do. Configure it in your MCP client as the command
`dinah mcp`, with the bench directory as the working directory or `DINAH_BENCH`
set to it. The server hands the client the bench's working agreement and a tool
per verb, so the agent claims, moves, releases and blocks under the same rules
and leaves the same journal entries you do.

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
