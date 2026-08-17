# Dinah quick start

In this guide you take one workbench from an empty directory to a finished card,
and you meet every command Dinah offers along the way. A workbench is the folder
of plain files that holds a board's work.

Every transcript below is real output from `dinah 0.1.0`. You will see absolute
paths in that output, written here as `C:\work\...` and `C:\Users\ana\...`; they
name the directories the session ran in, and nothing else in the output is
edited. A few blocks show you a file's contents rather than a transcript. You
type those into the file yourself, and each one says so where it appears.

Read the guide in order the first time. After that, use the section headings as
an index.

## Shells

You can run every command line below unchanged in bash, zsh, and PowerShell.
They use only plain arguments, relative paths, `mkdir`, and `cd`, and they leave
it to Dinah to find the workbench by walking up from the working directory. You
do two things differently in each of those shells: you set an environment
variable, and you substitute one command's output into another. You meet each of
those once below, written out as a labeled pair. One further
block uses a utility that only POSIX systems carry, and it too says so where it
appears.

The leading `$` marks a command line, so do not type it. This session ran on
Windows, so Dinah prints Windows paths in the output below. On macOS or Linux
you would see POSIX paths from the same runs.

## What Dinah tells you about itself

```
$ dinah version
dinah 0.1.0
conforms to dinah-core/1.0
storage format 1
[exit 0]
```

Dinah tells you three things there. The first line names the build you are
running. The second line names the shared rule set that build follows, and any
other tool built to those same rules can read this workbench and reach the same
answers about it. The third line names the format Dinah writes on disk.

`dinah help` lists all twenty-nine commands, in the four groups Dinah sorts them
into. Dinah does not accept `--help`:

```
$ dinah --help
dinah.usage --help was not understood; run dinah help for the surface
[exit 2]
```

Run `dinah help` instead, or run `dinah` with no arguments at all, and Dinah
prints you the same list of commands. To see one command's arguments and the
reasons it can say no, name that command: `dinah help move`.

## Open a workbench

A workbench is a directory of plain-text files. You may create a workbench in
the same directory as the rest of your work, if you'd like, and put it under
version control alongside the project it belongs to.

```
$ mkdir release-notes
$ cd release-notes

$ dinah init --slug rel --operator ana
Workbench created at C:\work\release-notes.
[exit 0]
```

Every card in a workbench has a human-readable prefix, called a slug. You passed
`rel` above, so the first card you file here will be `rel-1`, the second
`rel-2`, and so on. If you don't provide a slug with the `--slug` option, Dinah
will derive one from the directory name.

The operator owns the workbench and answers for it. Only the operator can lift a
block or force a move past a limit, so if you leave that seat empty, nobody can
perform those actions. If you don't name an operator with the `--operator`
option, Dinah records whoever you are acting as.

Dinah wrote you three things: `workbench.md`, a `states/` directory holding one
file per state, and a `.gitignore` that keeps Dinah's lock files out of your
commits. You have nothing else yet.

You run every command from here on inside the workbench directory, and you never
have to give any of them a path. Dinah finds the workbench by walking up from
wherever you are, the way git finds a repository. If that climb finds nothing,
Dinah tries one more place, the `.dinah` directory in your home, where you can
keep a workbench that belongs to you rather than to a project. You move that
directory elsewhere by setting `DINAH_HOME`, and the section on working from
outside a workbench comes back to it.

## Say who you are

Dinah assigns an owner to every action it takes, and it will not invent one for
you.

```
$ dinah whoami
no-owner no owner was resolvable; set one with --actor, DINAH_ACTOR or config actor
[exit 2]
```

The error message names the places where Dinah looked, in the order it looked
in them. It takes the `--actor` flag first, then the `DINAH_ACTOR` environment
variable, then your own configuration file. To avoid this error, you may set the
`actor` configuration value once:

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

`whoami` tells you who you are acting as and whether you are the operator. You
want that second answer, because Dinah lets the operator do things it will not
let anybody else do. Dinah keeps your settings in `config.md`, under `.dinah` in
your home directory. They belong to you rather than to the workbench, so they
follow you to every workbench you work.

If you do not give `config` an argument, Dinah lists every setting it knows, the
value each one currently resolves to, and where that value came from:

```
$ dinah config
  lang        en                      default
  actor       ana                     config
  editor      notepad                 fallback
[exit 0]
```

Read that third column whenever a value surprises you. You set none of `lang`,
so Dinah fell back to its own default; you wrote `actor` into your
configuration file a moment ago; and Dinah found `editor` on the machine rather
than taking a program you chose. Dinah labels an environment variable the same
way, so a value you cannot account for tells you where it came from:

```
$ dinah config
  lang        en                      default
  actor       bo                      environment
  editor      notepad                 fallback
[exit 0]
```

Dinah knows three settings in this version, `actor`, `lang`, and `editor`, and
it accepts no other name:

```
$ dinah config set colour green
dinah.unknown-key this tool knows no setting called colour
[exit 2]
```

Dinah will not set that setting, but it does not throw away names it does not
recognise when it rewrites the file. Anything you added there by hand stays
where you put it.

## Look at the flow

```
$ dinah states
  1d2f2a07a38b  intake                  Intake                          intake    0
  df8cd3d7f024  doing                   Doing                           work      0
  acd3a55081b7  done                    Done                            done      0
[exit 0]
```

You get one row per state, and each row gives you the state's identifier, its
slug, its title, its kind, and how many cards stand in it. A state's kind is
`intake`, `work`, or `done`. Dinah runs the flow in the order `workbench.md`
lists the states, so when you move a card to a later state you move it forward,
and when you move it to an earlier state you move it backward. Dinah generates
the identifiers per workbench, so yours will differ from the ones printed here.

You type the slug, which is the short name. Every command that takes a state
accepts the identifier, the slug, or the title, and Dinah ignores case when it
matches them. Dinah derives the slugs from the titles when it creates the
workbench, so if you have a workbench made before slugs existed, its states
carry none, and you fill them in with `dinah check --migrate-slugs`.

Run `status` when you sit down. Dinah prints you that same list and adds the
cards you hold yourself.

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

You edit a workbench by editing its files. Open `workbench.md`, give the
workbench a real title, and write your standing instructions below the settings
block at the top, the part between the `---` lines. Dinah's own messages call
that block the frontmatter. The block below is the file itself rather than a
transcript:

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

Then open the `state.md` of one state and do the same. If you set a `wip_limit`,
Dinah caps that state at that many cards:

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

Dinah never copies that prose anywhere. It serves the text out of the file you
wrote it in, so when you edit the file, every reader sees the change at once.
Check your work whenever you have been editing by hand:

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

You can see the limit in the count column, where `doing` now reads `0/1`. To
read the instructions a state serves without touching a card, ask for them:

```
$ dinah instructions doing

Instructions, this workbench:
Every card on this workbench ends with a line in the changelog.

Instructions, this state:
Work the card until it is finished or until something stops you.
Leave a comment saying what you did before you carry it on.
[exit 0]
```

If you give `dinah instructions` a card reference instead, Dinah serves you the
instructions for wherever that card is standing.

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

Dinah puts a new card in the first state of the flow, unless you name another
state with `--state`. Your card arrives with the substate `ready`, and anybody
may pull a ready card. You read where the card stands out of the bracket after
its title: first the state it stands in, then its substate.

You read the board back with three commands. `ls` lists cards, and takes an
optional state and an optional `--ready` filter. `next` tells you what each
state is offering right now. `show` prints one card. Dinah lists cards in queue
order, oldest arrival first, and when two cards arrived inside the same second
it falls back to the order you filed them in. You get that same order however
fast you type.

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

You change where a card stands with five commands: `claim`, `move`, `release`,
`block`, and `unblock`. Dinah's own guide calls these five the verbs, and you
can read it with `dinah guide verbs`. The shared rules fix what each one does,
so a second tool reading the same workbench answers you the same way.

Nobody hands you work here. You claim a card yourself:

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

When your claim succeeds, Dinah shows you the instructions for where the card
stands and the moves the flow allows from there, so you do not have to remember
either. Pass `--quiet` when you have read them already.

Dinah will not let anybody else take a card you hold:

```
$ dinah claim rel-1 --actor bo
held ana holds this card
[exit 2]
```

`move` carries a card to another state and changes nothing else, so if you move
a card you hold, you still hold it afterwards. You capped `Doing` at one card
above, and `rel-3` already stands there, so Dinah refuses the move below:

```
$ dinah move rel-1 doing
at-capacity state df8cd3d7f024 has reached its limit
[exit 2]
```

Only the operator can override that limit, and Dinah records the override on the
move:

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

If you pass a single dash instead of the text, `comment` reads standard input.
Your scripts use that to hand it something longer than a command line wants.

When something stops the work, say so on the card. Dinah frees the card and
records why you blocked it, so that everybody reading the board can see the
obstacle:

```
$ dinah block rel-2 "Waiting on the signing certificate" --kind external
rel-2  Draft the changelog  [Intake / blocked]
  blocked: Waiting on the signing certificate
[exit 0]
```

Dinah requires the reason, and you write it as prose, because the things that
stop real work vary too much for a fixed list. You choose the `--kind` yourself,
as a short label you can group blocks by later. You see your blocked cards in
`status`:

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

Only the operator can lift a block. When you block a card, you hand the obstacle
to whoever answers for the workbench:

```
$ dinah unblock rel-2 --actor bo
not-operator this act is the operator's, and you are bo
[exit 2]

$ dinah unblock rel-2
rel-2  Draft the changelog  [Intake / ready]
[exit 0]
```

Release a card as soon as you stop working it, and the queue will stay honest
about what is available. You can also give a claim its own expiry. If your claim
lapses, Dinah returns the card to the queue and records the lapse:

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

You can always move a card backward out of a done state, but if you try to move
one forward out of a done state, Dinah refuses with `terminal`. Dinah therefore
offers you only backward moves above.

## Everything below a card

Dinah stores a card as a directory, and its comments and attachments as
directories inside that one. When you give `attach` a file, Dinah copies the
bytes in under the file's own name. Create `notes.txt` in the current directory
first, with whatever content you like:

```
$ dinah attach rel-1 notes.txt
rel-1  Write the release notes  [Done / active]
  held by ana
[exit 0]
```

Pass `--replace` to swap the bytes of an attachment that is already there.

You address anything below a card with a path reference, which is the card's
reference followed by slash-separated segments. You write `rel-1/attachments/1`
for the attachment you just made. The segments you can use are `comments`,
`attachments`, `checklist`, `journal`, and `card`, plus `oq`, `ac`, and `d` as
shorthands for the three checklist kinds. If you reach past an attachment into
`payload`, you get the file itself. No command in this version files a checklist
item, so you can only address checklist items that something else has already
written.

You name a thing in a collection either by its twelve-hex identifier or by its
position, counting from one, and Dinah counts positions in the order the things
were written. Dinah's own messages call a comment, an attachment, or a checklist
item an entity, and it stamps every one of them with a creation ordinal. You can
see that as the `ordinal` line in the frontmatter below, and it is what keeps
`rel-1/comments/1` pointing at the comment you wrote first as others arrive.

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

If you have an older workbench, written before ordinals existed, its entities
carry none, and a position there is only as good as the directory listing. You
repair that with `dinah check --migrate-ordinals`, and the section on defects
below runs it.

Dinah keeps the full record of a card in its journal, and `log` shows you that
journal oldest first:

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

You can see the override on the move that used it. Dinah names each state in the
log as it was titled at the time, so if you rename a state later, your history
still reads as it did.

## Taking things out

`archive` takes a card, or a comment or attachment on one, out of the listings
and keeps its files. `delete` destroys the same things and their history. Dinah
says nothing to you when either one succeeds.

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

When you archive a card, Dinah moves it under `archive/cards/` in the workbench
and stops listing it. You cannot recover anything you delete, so `delete` makes
you pass `--yes`.

## When the files are wrong

You may edit the files by hand, and when you do you can make mistakes in them,
so run `check` to find them. The workbench answering below is not the one you
have been working in. It was damaged on purpose for this example, and a card in
it names a state that no longer exists:

```
$ dinah check
  a card names state 000000000000, which this workbench does not declare (C:\work\release-notes\cards\eafed105f127\card.md)
  the journal puts this card in state 1d2f2a07a38b, and its frontmatter disagrees (C:\work\release-notes\cards\eafed105f127\card.md)
2 defects.
[exit 2]
```

Dinah names the file to open on every line. Open each one in your editor, fix
it, and run `check` again:

```
$ dinah check
No structural defects found.
[exit 0]
```

`check` also catches a claim without the substate that implies it, a block with
no reason, a link pointing at no card, a journal whose last line was cut off,
and a directory sitting where an entity should be. It only reads and reports,
and it changes nothing unless you ask it to.

`check` reports two things that mark an older workbench rather than a mistake,
and you repair each of them with a flag. The workbench answering below is that
older one, kept for this example rather than the workbench you have been
building, so your own `check` still reports nothing at this point. A state
written before slugs existed carries no slug, and an entity written before
ordinals existed carries no ordinal:

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

Dinah prints what each migration wrote and then reports whatever it did not fix,
so you keep getting exit code 2 until the workbench is clean. Dinah takes the
slugs from the titles and reads the ordinals back out of each card's journal, so
keep your journals intact. If the workbench matters to you, run either migration
against a copy first.

You use a third flag, `--finish`, in a rarer case. If a power cut or a killed
process interrupts Dinah part way through a structural act, Dinah leaves a lock
file behind naming what it was doing, and `check` reports that as an interrupted
act. `check --finish` reads the journal to decide whether the act reached its
point of record, then completes it or rolls it back. Run `check` after any hand
edit, and before you commit a workbench to version control.

## Driving Dinah from a script

Dinah exits with one of four codes, and your script should tell them apart,
because each one asks something different of you.

| Code | Outcome | What to do |
| --- | --- | --- |
| 0 | ok | It happened. |
| 2 | refused | A rule said no. Dinah names the rule at the front of the message. |
| 3 | stale | The card moved between your reading it and your acting. Read it again and retry. |
| 4 | unreachable | The question could not be asked at all. |

Pass `--json` and Dinah gives you the machine-readable form. When Dinah says no
under `--json`, it writes the name of the rule to standard output in the
`refusal` field, and that is the portable way for your script to find out which
rule said no. Here is standard output alone:

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

Dinah writes the sentence a person reads to standard error, whether or not you
passed `--json`. If you do not pass the flag, that one line is all you get, and
Dinah puts the name of the rule at the front of it:

```
$ dinah claim rel-9
unknown-card this workbench carries no card rel-9
[exit 2]
```

Dinah writes the sentence after the name for a person to read, and it may
translate that sentence. It never translates the name. In bash or zsh, you cut
that first word out:

```
$ dinah claim rel-9 2>&1 >/dev/null | cut -d' ' -f1
unknown-card
```

That block runs only on POSIX systems. Branch on Dinah's own exit code, so
capture it before a pipe puts another program's status in its place. PowerShell
also decorates a native command's error stream when it redirects one, so take
the `--json` route above when you want the same behaviour everywhere.

Dinah takes some of those names from the shared rules that every
Dinah-compatible tool follows, and coins the rest itself. Dinah puts a `dinah.`
prefix on the names it coins, so your script tells the two groups apart by
matching on that prefix. `unknown-card` and `at-capacity` come from the shared
rules. `dinah.unknown-key` and `dinah.unconfirmed` are Dinah's own.

If you set `DINAH_FORMAT=json`, Dinah gives you the machine-readable form from
every call without the flag. You set an environment variable differently in each
shell, and this is the first of the two places where that matters.

In bash or zsh:

```
export DINAH_FORMAT=json
```

In PowerShell:

```
$env:DINAH_FORMAT = 'json'
```

Dinah writes the machine spellings in JSON, such as `ready` and `unknown-card`,
and never translates them, so you get the same bytes from the same command under
any language setting.

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

The `revision` is the card's content as you read it, reduced to a hash. Dinah
measures the stale outcome against it, so if your tool reads a card, thinks, and
then acts, it can find out that the card moved in between.

You compose `path` with other commands. It writes one absolute path to standard
output and nothing else. You substitute that into another command differently in
each shell, and this is the second of the two places where that matters.

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
`DINAH_LANG`, then your own configuration file, then the operating system
locale, and English if none of them answers. Dinah puts the locale below your
configuration file because the locale describes the machine rather than the
person reading the screen.

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

You wrote the titles, so Dinah leaves them as you wrote them, and it leaves the
slugs derived from them alone as well. To see which languages your build
carries, ask:

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

If you ask for a language whose catalog has no entries, Dinah falls back to
English message by message, so you see no change yet.

## Open a card in your editor

`edit` accepts the same references `path` does and hands the file to your
editor. Dinah looks at `DINAH_EDITOR` first, then your own configuration file,
then `VISUAL`, then `EDITOR`, and falls back to a platform default. Dinah puts
its own variable on top because you share `EDITOR` with every other tool you
run, so if you want git and Dinah to open different editors, you have nowhere
else to say so.

Set `DINAH_EDITOR` the way your shell sets any variable, as the scripting
section above shows. The run below points it at `more`, so you see the file
printed instead of a window opening:

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

Dinah runs that value as a program name, so give it the name of an editor rather
than a command line with flags in it:

```
$ dinah config set editor notepad
[exit 0]

$ dinah config get editor
notepad
[exit 0]
```

If you set none of those and Dinah finds no fallback editor on the machine,
`edit` fails with `dinah.no-editor`.

## Work on a workbench you are not standing in

Dinah walks up from the working directory to find its workbench, so you can
stand anywhere inside one. To see what that walk reaches from where you are
standing now, run `workbenches`. Dinah answers you with rows rather than an
error, and it tells you plainly when it reached none:

```
$ dinah workbenches
  Release 0.2                     rel             C:\work\release-notes
[exit 0]

$ cd ..

$ dinah workbenches
no workbench is reachable from here
[exit 0]
```

Dinah only climbs, so it reaches a workbench above you and never one in a
directory beside you. From outside, name the workbench you want with
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

You can keep workbenches of your own in your home directory, in the `.dinah`
directory beside your settings, and Dinah falls back to those when the climb
finds nothing. If you keep two or more of them there, Dinah will not choose
between them for you. The listing below comes from ana's own home directory,
which holds two workbenches for this example. You will see the same thing only
if you have made more than one workbench there yourself:

```
$ dinah workbenches
  0f1e2d3c4b5a                    bet             C:\Users\ana\.dinah\0f1e2d3c4b5a
  a1b2c3d4e5f6                    alp             C:\Users\ana\.dinah\a1b2c3d4e5f6
[exit 0]

$ dinah status
dinah.ambiguous-bench 0f1e2d3c4b5a (C:\Users\ana\.dinah\0f1e2d3c4b5a); a1b2c3d4e5f6 (C:\Users\ana\.dinah\a1b2c3d4e5f6) are all reachable from C:\Users\ana\.dinah; choose one with --workbench <path>, or run from inside it
[exit 2]
```

If you run `show` with no argument, Dinah prints that same listing. You asked it
to show you something, it cannot tell which workbench you meant, so it shows you
the choices. If Dinah reaches no workbench at all, it fails with
`dinah.no-bench-found` and names both the directory the climb started from and
the home directory it fell back to.

You move that fallback directory by setting `DINAH_HOME`. If you point it
somewhere else, Dinah reads your settings file and any workbenches under it from
there instead. Set it when you want to work against a scratch setup without
touching your own.

## Hand a workbench to somebody else

`export` prints the whole workbench definition in the shared exchange format,
and another program built to the same rules can read what it prints:

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

`extract` writes that same definition to a directory as a template you can use
again. It carries the flow and the instructions, and none of the cards. You
start a new workbench from a template with `init --from`:

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

The template carries the state identifiers and the slugs too, so you name the
states in the new workbench exactly as you name them in the old one. That last
`cd` puts you back in the workbench this guide started in, and the commands
below expect you to run them there.

## The guides that ship inside Dinah

Dinah carries its own guides, so you can read them with no network and no
repository checkout:

```
$ dinah guide
  getting-started     Getting started
  verbs               The five verbs
  workbench-layout    What a workbench looks like on disk
[exit 0]
```

Read `dinah guide workbench-layout` before you start editing files by hand,
because it maps the whole directory for you.

## Point an agent at the workbench

`dinah mcp` serves the workbench over MCP on its standard input and output, so
an AI colleague can work the same board you do. Configure it in your MCP client
as the command `dinah mcp`, and either run it with the workbench directory as
its working directory or set `DINAH_WORKBENCH` to that directory. Dinah hands
the client the rules for working this workbench and one tool for each command,
so your AI colleague claims, moves, releases, and blocks under the same rules
and leaves the same journal entries you do.

Give your AI colleague an actor name of its own through `DINAH_ACTOR`, so the
record shows who did what.

## Where to look next

`dinah help <command>` lists a command's arguments and every reason it can say
no, in the order Dinah makes the checks. Read that order when you are working
out which of two possible errors you are looking at.

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
