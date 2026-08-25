# Dinah quick start

In this guide you take one workbench from an empty directory to a finished card,
and you meet every command Dinah offers along the way. A workbench is the folder
of plain files that holds a board's work.

Every transcript below is real output from a published build of Dinah. The
session ran on Linux, under a home directory belonging to a person called ana,
and no character of its output is edited. Windows readers lose little by that choice,
since the commands are the same and Dinah prints Windows paths where this
session prints POSIX ones. Where a step really does differ between platforms,
this guide gives you both forms. A few blocks
show you a file's contents, or a command to type, rather than a transcript, and
each one says so where it appears.

Read the guide in order the first time. After that, use the section headings as
an index.

## Install Dinah

Dinah is one binary with no installer to click through and nothing else to set
up. On Linux or macOS, run this:

```console skip=the first word is curl, which the in-process head cannot run
$ curl -fsSL https://raw.githubusercontent.com/paulmooreparks/dinah/main/scripts/install.sh | sh
Installed dinah-linux-amd64 as /home/ana/.local/bin/dinah

You installed dinah to /home/ana/.local/bin, but this shell does not have that directory on PATH yet.
Debian and Ubuntu add ~/.local/bin to PATH automatically the next time a login shell starts, but only once the directory exists, and it did not exist before this install created it. Log out and back in to pick it up, or run this now to use dinah in this shell:
    export PATH="$HOME/.local/bin:$PATH"
Add that line to your shell's startup file (~/.profile, ~/.bashrc, ~/.zshrc, or wherever you keep it) to make it permanent.
[exit 0]
```

On Windows, run this in PowerShell instead:

```
irm https://raw.githubusercontent.com/paulmooreparks/dinah/main/scripts/install.ps1 | iex
```

Either script fetches the binary your machine needs, checks it against its
published SHA-256 before installing anything, and puts it somewhere you can
write without administrator privilege. On Linux and macOS that is
`~/.local/bin`, and on Windows it is `%LOCALAPPDATA%\dinah\bin`.

The two platforms part company over your PATH. On Windows the installer adds
`%LOCALAPPDATA%\dinah\bin` to your user PATH for you, and the next shell you
open finds `dinah` by name. If you would rather keep your PATH as it is, set
`DINAH_NO_PATH` before you run the one-liner. On Linux and macOS the installer
changes nothing and prints the line to add instead, which is what you see at the
end of the transcript above.

If you would rather not pipe a script into a shell, download your platform's
binary and `SHA256SUMS.txt` from the [releases
page](https://github.com/paulmooreparks/dinah/releases) and check the download
yourself before you run it. The command to type is not a transcript, and it
differs on all three platforms. On Linux:

```
sha256sum -c SHA256SUMS.txt --ignore-missing
```

On macOS:

```
shasum -a 256 -c SHA256SUMS.txt --ignore-missing
```

Windows gives you nothing that reads a checksum file. There you compute the hash
yourself and compare it by eye against the line for your binary. Use `certutil`,
which ships with Windows and runs the same way from `cmd.exe` or PowerShell:

```
certutil -hashfile dinah-windows-amd64.exe SHA256
```

## Shells

You can run every command line below unchanged in bash, zsh, and PowerShell.
They use only plain arguments, relative paths, `mkdir`, and `cd`, and they leave
it to Dinah to find the workbench by walking up from the working directory. You
do two things differently in each of those shells: you set an environment
variable, and you substitute one command's output into another. You meet each of
those once below, written out as a labeled pair. One further block uses a
utility that only POSIX systems carry, and it too says so where it appears.

The leading `$` marks a command line. Do not type it.

## What Dinah tells you about itself

```console
$ dinah version
dinah 0.1.0
conforms to dinah-core/0.5
storage format 1
[exit 0]
```

Dinah tells you three things there. The first line names the build you are
running, and yours will carry a later number than the one above, because every
release publishes a new one. The second line names the shared rule set that build follows, and any
other tool built to those same rules can read this workbench and reach the same
answers about it. The third line names the format Dinah writes on disk.

`dinah help` lists all forty commands, in the four groups Dinah sorts
them into. Running `dinah` with no arguments at all prints the same list. So
does whichever spelling of the help flag you already have the habit of typing,
because Dinah answers to `--help`, `-help`, `-h`, `-?`, `--?` and `/?` alike.

To see one command's arguments and the reasons it can say no, name that
command. Both `dinah help move` and `dinah move --help` print that page, and
you may put the flag on either side of the command name.

The version flags work the same way. `dinah --version`, `-version`, `-V` and
`-v` each print what `dinah version` printed above.

## Open a workbench

A workbench is a directory of plain-text files. You may create a workbench in
the same directory as the rest of your work, if you'd like, and put it under
version control alongside the project it belongs to.

```console
$ mkdir release-notes
$ cd release-notes
$ dinah init --slug rel --operator ana
Workbench created at /home/ana/release-notes/.dinah/d0e41d414bb5.
Dinah recorded you as actor ana in /home/ana/.dinah/config.md, and will record that name on everything you do. Run `dinah config set actor <name>` to change it, or `dinah config set actor` to clear it.
[exit 0]
```

Dinah put everything it wrote in one place rather than scattering it through the
directory you were standing in. It made a `.dinah` directory there, and inside
that one more directory named with the workbench's own twelve-hex identifier,
and everything it writes from now on lives under that. Inside it you have
`workbench.md`, a `states/` directory holding one directory per state with a
`state.md` inside it, and a `.gitignore` that keeps Dinah's lock files out of
your commits. You have nothing else yet.

Every card in a workbench has a human-readable prefix, called a slug. Because
you passed `rel` above, the first card you file here will be `rel-1`, the second
`rel-2`, and so on. If you don't provide a slug with the `--slug` option, Dinah
will derive one from the directory name.

The operator owns the workbench and answers for it. Only the operator can lift a
block or force a move past a limit. If you leave that seat empty, nobody can
perform those actions. If you don't name an operator with the `--operator`
option, Dinah records whoever you are acting as.

You run every command from here on inside the workbench directory, and you never
have to give any of them a path. Dinah finds the workbench by walking up from
wherever you are, the way git finds a repository. If that climb finds nothing,
Dinah tries one more place, the `.dinah` directory in your home, where you can
keep a workbench that belongs to you rather than to a project. You move that
directory elsewhere by setting `DINAH_HOME`, and the section on working from
outside a workbench comes back to it.

## Say who you are

Dinah recorded you as the actor when it created the workbench above, because
nothing on your machine had named an owner yet. Dinah looks for that name on
the `--actor` flag first, then in the `DINAH_ACTOR` environment variable, then
in your own configuration file. You change the recorded name with
`dinah config set actor`, and you use the same command to act as somebody other
than the operator:

```console
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
your home directory. They belong to you rather than to the workbench, and they
follow you to every workbench you work.

If you do not give `config` an argument, Dinah lists every setting it knows, the
value each one currently resolves to, and where that value came from:

```console skip=the editor fallback differs per platform and per runner image
$ dinah config
  Setting    Value                                        Source
  ---------  -------------------------------------------  --------
  lang       en                                           default
  actor      ana                                          config
  editor     nano                                         fallback
  workbench  /home/ana/release-notes/.dinah/d0e41d414bb5  search
[exit 0]
```

You ran that on Linux. On Windows, Dinah falls back to `notepad`, so you see
that in the `editor` row instead.

Read that third column whenever a value surprises you. You set none of `lang`,
and Dinah fell back to its own default; you wrote `actor` into your
configuration file a moment ago; Dinah found `editor` on the machine rather than
taking a program you chose; and `workbench` is not a setting you wrote at all,
but the workbench this run resolved, marked `search` because the climb found it.
Dinah labels an environment variable the same way. A value you cannot account
for tells you where it came from:

```console skip=the editor fallback differs per platform and per runner image
$ dinah config
  Setting    Value                                        Source
  ---------  -------------------------------------------  -----------
  lang       en                                           default
  actor      bo                                           environment
  editor     nano                                         fallback
  workbench  /home/ana/release-notes/.dinah/d0e41d414bb5  search
[exit 0]
```

On Windows you see `notepad` here instead.

Dinah knows four settings in this version, `actor`, `lang`, `editor`, and
`workbench`, and it accepts no other name:

```console
$ dinah config set colour green
dinah.unknown-key Dinah knows no setting or field called colour; it knows these
  lang
  actor
  editor
  workbench
name one of them instead, or run `dinah config` to see what each one holds
[exit 2]
```

Dinah will not set that setting, but it does not throw away names it does not
recognise when it rewrites the file. Anything you added there by hand stays
where you put it.

## Look at the flow

```console
$ dinah states
  Slug    Name    Kind    Cards  Work        Owner
  ------  ------  ------  -----  ----------  -----
  intake  Intake  intake  0      none taken  agent
  doing   Doing   work    0      taken       agent
  done    Done    done    0      none taken  agent
[exit 0]
```

You get one row per state. The slug is the short name you type, the name is what
you called the state, the kind is one of `intake`, `work`, `done`, and `dinah.buffer`, and the
count is how many cards stand there. Dinah runs the flow in the order
`workbench.md` lists the states. When you move a card to a later state you move
it forward, and when you move it to an earlier state you move it backward.

The last column says who may move a card out of the state. It reads `agent` for
a state anybody can work and `operator` for one where the departure is the
operator's alone, and you choose the second by writing `operator_owned: true`
into the state's own file. Every state starts out an agent's.

The column before it says whether work is taken up at the state at all, and it
reads one of three things. It reads `taken` for a state where somebody works a
card: you claim the card there, work it, and move it on.

It reads `waiting` for a state where the workbench is waiting on somebody
outside it: a reviewer, a customer, a supplier. You choose that by writing
`awaiting_outside: true` into the state's own file. Nobody claims a card there,
`dinah next` offers nothing from it, and `dinah pull` neither takes a card out
of it nor lands one in it, but a card standing there is ready in the ordinary
way, carries no block, and anybody may move it on when the answer comes.

It reads `none taken` for a state where nobody works a card and a card is
waiting for the state beyond rather than for a person. An intake state and a
done state both read this way, and so does a state you mark `kind: dinah.buffer`
to hold cards between two stations. You cannot claim a card at such a state, and
you cannot move a card you are holding into one, so you release it first. What
you can do is pull the card into the station beyond, which is what a card
standing there is waiting for. A block still works wherever a card stands,
because a block says something about the card rather than about a worker.

The two columns answer different questions, so a state can be either, both, or
neither.

If you have been keeping such cards out of the queue by blocking them, the way
across is short and there is no migration command, because nothing can tell a
workaround block from a real one. Write `awaiting_outside: true` into the
state's file, then run `dinah unblock <card>` as the operator once for each card
you had blocked to keep it out of the way. Each card returns to `ready`, the
unblock is journaled so the record says when the workaround ended, and the flag
keeps the cards out of the ready queue from that moment.

Dinah also gives each state a twelve-hex identifier, which you will meet in
`workbench.md`, in `export`, and in the card files themselves, though no listing
prints it at you. Every command that takes a state accepts the identifier, the
slug, or the title, and Dinah ignores case when it matches them. Dinah derives
the slugs from the titles when it creates the workbench. If you have a workbench
made before slugs existed, though, its states carry none, and you fill them in
with `dinah check --migrate-slugs`.

Run `status` when you sit down. Dinah prints you that same list, tells you which
workbench it resolved and how, and adds the cards you hold yourself.

```console
$ dinah status
release-notes  (/home/ana/release-notes/.dinah/d0e41d414bb5)  [search]
acting as ana, operator: yes

  Slug    Name    Kind    Cards  Work        Owner
  ------  ------  ------  -----  ----------  -----
  intake  Intake  intake  0      none taken  agent
  doing   Doing   work    0      taken       agent
  done    Done    done    0      none taken  agent
[exit 0]
```

## Edit the workbench by hand

You edit a workbench by editing its files. Open `workbench.md`, give the
workbench a real title, and write your standing instructions below the settings
block at the top, the part between the `---` lines. Dinah's own messages call
that block the frontmatter. The block below is the file itself rather than a
transcript:

```file path=<workbench>/workbench.md
---
format: 1
profile: dinah-core/0.5
title: Release 0.2
slug: rel
operator: ana
states:
  - 003b09ee6e31
  - 780659205f6b
  - fcd0d92e167a
---
Every card on this workbench ends with a line in the changelog.
```

Then open the `state.md` of one state and do the same. If you set a `wip_limit`,
Dinah caps that state at that many cards:

```file path=<workbench>/states/<state:doing>/state.md
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
wrote it in. When you edit the file, every reader sees the change at once. Check
your work whenever you have been editing by hand:

```console
$ dinah check
No structural defects found.
[exit 0]
$ dinah status
Release 0.2  (/home/ana/release-notes/.dinah/d0e41d414bb5)  [search]
acting as ana, operator: yes

  Slug    Name    Kind    Cards  Work        Owner
  ------  ------  ------  -----  ----------  -----
  intake  Intake  intake  0      none taken  agent
  doing   Doing   work    0/1    taken       agent
  done    Done    done    0      none taken  agent
[exit 0]
```

You can see the limit in the count column, where `doing` now reads `0/1`. To
read the instructions a state serves without touching a card, ask for them:

```console
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

```console
$ dinah add "Write the release notes"
rel-1  Write the release notes  [Intake / ready]
[exit 0]
$ dinah add "Draft the changelog"
rel-2  Draft the changelog  [Intake / ready]
[exit 0]
$ dinah add "Check the download links"
rel-3  Check the download links  [Intake / ready]
[exit 0]
```

Dinah puts a new card in the first state of the flow. Your card arrives with
the substate `ready`, and anybody may pull a ready card. You read where the
card stands out of the bracket after its title: first the state it stands in,
then its substate.

All three cards are standing in `Intake`, so nothing has gone anywhere yet and
you have not yet seen a state as somewhere a card travels to. Send one of them
on:

```console
$ dinah move rel-3 doing
rel-3  Check the download links  [Doing / ready]

Instructions, this workbench:
Every card on this workbench ends with a line in the changelog.

Instructions, this state:
Work the card until it is finished or until something stops you.
Leave a comment saying what you did before you carry it on.

Moves this card may make:
  State   Name    Direction
  ------  ------  ---------
  intake  Intake  backward
  done    Done    forward
[exit 0]
```

`move` carries a card to the state you name and changes nothing else about it.
`rel-3` now stands in `Doing`, still waiting for somebody to pull it, and Dinah
shows you the instructions that state serves along with the moves the card may
make from there. You have met the first of the five commands that change where
a card stands, and you meet the other four under "The five commands underneath"
below. If you already know as you file a card that it belongs somewhere other
than the first state, you can name that state with `--state`, as in `dinah add
"Check the download links" --state doing`, and reach the same place in one
command instead of two.

You read the board back with three commands. `ls` lists cards, and takes an
optional state and an optional `--ready` filter. `next` tells you what each
state is offering right now. `show` prints one card. Dinah lists cards in queue
order, oldest arrival first, and when two cards arrived inside the same second
it falls back to the order you filed them in. You get that same order however
fast you type.

```console
$ dinah ls
  Card   Standing  Title
  -----  --------  ------------------------
  rel-1  ready     Write the release notes
  rel-2  ready     Draft the changelog
  rel-3  ready     Check the download links
[exit 0]
$ dinah ls doing
  Card   Standing  Title
  -----  --------  ------------------------
  rel-3  ready     Check the download links
[exit 0]
$ dinah ls intake --ready
  Card   Standing  Title
  -----  --------  -----------------------
  rel-1  ready     Write the release notes
  rel-2  ready     Draft the changelog
[exit 0]
$ dinah next
  State   Card   Title                     Take
  ------  -----  ------------------------  -----
  Intake  rel-1  Write the release notes   pull
  Doing   rel-3  Check the download links  claim
  Done    nothing is taken from here
[exit 0]
$ dinah next doing
  State  Card   Title                     Take
  -----  -----  ------------------------  -----
  Doing  rel-3  Check the download links  claim
[exit 0]
$ dinah show rel-1
rel-1  Write the release notes  [Intake / ready]
[exit 0]
```

## Take a card, and carry it on

A workbench is a pull system, which means nobody hands you work. You take the
next card yourself, and `dinah pull` is the one command that does it. Name a
state, and Dinah takes the card at the head of the state before it, moves it
in, and claims it for you.

You capped `Doing` at one card earlier, and `rel-3` is already standing there,
so Dinah refuses to pull another card into it:

```console
$ dinah pull doing
at-capacity state doing has reached its limit; move a card out of doing first, or raise that state's wip_limit
[exit 2]
```

Only the operator can carry a card through a full state, and Dinah records the
override:

```console
$ dinah pull doing --override
rel-1  Write the release notes  [Doing / active]
  held by ana

Instructions, this workbench:
Every card on this workbench ends with a line in the changelog.

Instructions, this state:
Work the card until it is finished or until something stops you.
Leave a comment saying what you did before you carry it on.

Moves this card may make:
  State   Name    Direction
  ------  ------  ---------
  intake  Intake  backward
  done    Done    forward
[exit 0]
```

Dinah picked `rel-1` because it stood at the head of the queue in `Intake`, the
state before `Doing`. That is the card `dinah next intake` was offering you a
moment ago, and pull and next read the queue the same way. When a claim
succeeds, Dinah shows you the instructions for where the card now stands and
the moves the flow allows from there, so you do not have to remember either.
Pass `--quiet` when you have read them already.

You can leave the state out. Dinah then works out which state you could pull
into and uses that one, and when more than one qualifies it stops and asks you
to name one rather than choosing for you. What qualifies depends on what is
standing on the workbench at the moment you type the command, and on who you
are, so the bare form can mean one thing today and another tomorrow. Run
`dinah help pull` for the whole list of what Dinah checks before it moves a
card.

Add `--no-claim` when you want to carry a card forward and leave it for
somebody else. The card lands in the new state still waiting, and the next
person to pull that state's own queue takes it from there.

## The five commands underneath

Under that one command Dinah did two things, and the card's own history shows
both of them:

```console
$ dinah log rel-1
  When                  Action   Actor  Detail
  --------------------  -------  -----  --------------------------
  2026-01-05T09:00:00Z  created  ana    Write the release notes
  2026-01-05T09:00:00Z  claimed  ana
  2026-01-05T09:00:00Z  moved    ana    Intake to Doing (override)
[exit 0]
```

Five commands change where a card stands: `claim`, `move`, `release`, `block`,
and `unblock`. Dinah's own guide calls these five the verbs, and you can read it
with `dinah guide verbs`. The shared rules fix what each one does. A second tool
reading the same workbench answers you the same way.

`claim` takes up a card you name, where it already stands, and `move` carries
it on. `rel-3` is standing in `Doing`, so you can take it up there:

```console
$ dinah claim rel-3
rel-3  Check the download links  [Doing / active]
  held by ana

Instructions, this workbench:
Every card on this workbench ends with a line in the changelog.

Instructions, this state:
Work the card until it is finished or until something stops you.
Leave a comment saying what you did before you carry it on.

Moves this card may make:
  State   Name    Direction
  ------  ------  ---------
  intake  Intake  backward
  done    Done    forward
[exit 0]
```

Dinah will not let anybody else take a card you hold:

```console
$ dinah claim rel-3 --actor bo
held ana holds this card; wait for ana to release it
[exit 2]
```

`release` gives a card back. Give one back the moment you stop working it,
because a card you are still holding is a card nobody else will pull:

```console
$ dinah release rel-3
rel-3  Check the download links  [Doing / ready]
[exit 0]
```

You already ran `move` once, to carry `rel-3` into `Doing`, so what follows is
the fuller account rather than a first introduction. `move` carries a card to
another state and changes nothing else. If you move a card you hold, you still
hold it afterwards. `move` obeys the same capacity limit `pull` obeyed above,
and the operator overrides it the same way.

Say what you did while you are there:

```console
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
records why you blocked it. Everybody reading the board can then see the
obstacle:

```console
$ dinah block rel-2 "Waiting on the signing certificate" --kind external
rel-2  Draft the changelog  [Intake / blocked]
  blocked: Waiting on the signing certificate
[exit 0]
```

Dinah requires the reason, and you write it as prose, because the things that
stop real work vary too much for a fixed list. You choose the `--kind` yourself,
as a short label you can group blocks by later. You see your blocked cards in
`status`:

```console
$ dinah status
Release 0.2  (/home/ana/release-notes/.dinah/d0e41d414bb5)  [search]
acting as ana, operator: yes

  Slug    Name    Kind    Cards  Work        Owner
  ------  ------  ------  -----  ----------  -----
  intake  Intake  intake  1      none taken  agent
  doing   Doing   work    2/1    taken       agent
  done    Done    done    0      none taken  agent

You are holding:
  Card   Title
  -----  -----------------------
  rel-1  Write the release notes

Blocked, waiting on the operator:
  Card   Reason
  -----  ----------------------------------
  rel-2  Waiting on the signing certificate
[exit 0]
```

Only the operator can lift a block. When you block a card, you hand the obstacle
to whoever answers for the workbench:

```console
$ dinah unblock rel-2 --actor bo
not-operator this action is the operator's, and you are bo; ask the operator to run it, or run `dinah whoami` to see who Dinah takes you to be
[exit 2]
$ dinah unblock rel-2
rel-2  Draft the changelog  [Intake / ready]
[exit 0]
```

Release a card as soon as you stop working it, and the queue will stay honest
about what is available. You can also give a claim its own expiry. If your claim
lapses, Dinah returns the card to the queue and records the lapse:

```console
$ dinah release rel-1
rel-1  Write the release notes  [Doing / ready]
[exit 0]
$ dinah claim rel-1 --expires 8h --quiet
rel-1  Write the release notes  [Doing / active]
  held by ana
[exit 0]
$ dinah release rel-1
rel-1  Write the release notes  [Doing / ready]
[exit 0]
$ dinah move rel-1 done
rel-1  Write the release notes  [Done / ready]

Instructions, this workbench:
Every card on this workbench ends with a line in the changelog.

Moves this card may make:
  State   Name    Direction
  ------  ------  ---------
  intake  Intake  backward
  doing   Doing   backward
[exit 0]
```

Nobody takes work up at a done state, so Dinah will not carry a card you are
holding into one. Release the card first, as the transcript does, and then move
it. The card arrives ready and nobody holds it, which is what a finished card
looks like.

You can always move a card backward out of a done state, but if you try to move
one forward out of a done state, Dinah refuses with `terminal`. Dinah therefore
offers you only backward moves above.

## Everything below a card

Dinah stores a card as a directory, and its comments and attachments as
directories inside that one. When you give `attach` a file, Dinah copies the
bytes in under the file's own name. Create `notes.txt` in the current directory
first. The block below is that file rather than a transcript:

```file path=notes.txt
Ask the signing team for the certificate.
```

Then attach it to the card:

```console
$ dinah attach rel-1 notes.txt
rel-1  Write the release notes  [Done / ready]
[exit 0]
```

If you want to change the bytes of an attachment later, pass `--replace` and
give `attach` the attachment's own reference rather than the card's:
`dinah attach rel-1/attachments/1 notes.txt --replace`. If you give `attach`
the card's reference instead, Dinah writes a second attachment and exits 0
without telling you that `--replace` did nothing, and you are left with two
copies of the file. The paragraph below explains how a reference like
`rel-1/attachments/1` is built.

If you want to rename an attachment without rewriting its bytes, give
`rename` the attachment's own reference and the new filename:
`dinah rename rel-1/attachments/1 cert-notes.txt`. Dinah renames the file
under `payload/` to match, rewrites the anchor's `filename` to match, and
appends one `attachment_renamed` line to the journal. A rename to the name
the attachment already carries exits 0 and appends no line, so replaying a
script does not fill the journal with rows recording nothing.

You address anything below a card with a path reference, which is the card's
reference followed by slash-separated segments. You write `rel-1/attachments/1`
for the attachment you just made. The segments you can use are `comments`,
`attachments`, `checklist`, `journal`, and `card`, plus `oq`, `ac`, and `d` as
shorthands for the three checklist kinds. If you reach past an attachment into
`payload`, you get the file itself. No command in this version files a checklist
item. You can only address checklist items that something else has already
written.

You name a thing in a collection either by its twelve-hex identifier or by its
position, counting from one, and Dinah counts positions in the order the things
were written. Dinah stamps every comment, attachment, and checklist item with a
creation ordinal. You see that ordinal in the frontmatter below, and it is what
keeps `rel-1/comments/1` pointing at the comment you wrote first as others
arrive.

```console
$ dinah rename rel-1/attachments/1 cert-notes.txt
rel-1  Write the release notes  [Done / ready]
[exit 0]
$ dinah show rel-1/attachments/1
---
filename: cert-notes.txt
provenance: ana
ordinal: 1
---
[exit 0]
$ dinah show rel-1/comments/1
---
ts: 2026-08-18T21:02:23Z
author: ana
ordinal: 1
---
Drafted entries for the four merged branches.
[exit 0]
$ dinah show rel-1/comments/2
---
ts: 2026-08-18T21:02:23Z
author: ana
ordinal: 2
---
Second half needs the signing certificate.
[exit 0]
$ dinah path rel-1/attachments/1/payload
/home/ana/release-notes/.dinah/d0e41d414bb5/cards/73ca475d0aaa/attachments/fcd92b769691/payload/cert-notes.txt
[exit 0]
```

If you have an older workbench, written before ordinals existed, its comments
and attachments carry no ordinal, and a position there is only as good as the
directory listing. You repair that with `dinah check --migrate-ordinals`, and
the section on defects below runs it.

Dinah keeps the full record of a card in its journal, and `log` shows you that
journal oldest first:

```console
$ dinah path rel-1
/home/ana/release-notes/.dinah/d0e41d414bb5/cards/73ca475d0aaa/card.md
[exit 0]
$ dinah path rel-1/journal
/home/ana/release-notes/.dinah/d0e41d414bb5/cards/73ca475d0aaa/journal.ndjson
[exit 0]
$ dinah log rel-1
  When                  Action              Actor  Detail
  --------------------  ------------------  -----  ---------------------------
  2026-08-18T21:02:23Z  created             ana    Write the release notes
  2026-08-18T21:02:23Z  claimed             ana
  2026-08-18T21:02:23Z  moved               ana    Intake to Doing (override)
  2026-08-18T21:02:23Z  commented           ana
  2026-08-18T21:02:23Z  commented           ana
  2026-08-18T21:02:23Z  released            ana
  2026-08-18T21:02:23Z  claimed             ana
  2026-08-18T21:02:23Z  released            ana
  2026-08-18T21:02:23Z  moved               ana    Doing to Done
  2026-08-18T21:02:23Z  attached            ana    notes.txt
  2026-08-18T21:02:23Z  attachment renamed  ana    notes.txt to cert-notes.txt
[exit 0]
```

You can see the override on the move that used it. Dinah names each state in the
log as it was titled at the time. If you rename a state later, your history
still reads as it did.

## Group cards into a workstream

A workstream is a named grouping of cards inside one workbench, and you use it
when several efforts run through the same flow at once. It has a title, a short
handle called a slug, a status, and a body you write your own notes in. It does
not change how a card moves: nothing Dinah refuses, orders, or counts depends
on which workstream a card belongs to.

You create one with `dinah workstream new`, and Dinah derives the slug from the
title. Quote a title of more than one word, the way you quote a card title:

```console
$ dinah workstream new "Autumn release"
autumn-release  Autumn release  [active]
[exit 0]
```

The derived slug is the whole title, so this workstream answers to
`autumn-release`. If you want a shorter handle, write one. A slug change needs
`--yes`, because every reference to the workstream you have written down
elsewhere names the old one:

```console
$ dinah workstream set autumn-release slug autumn --yes
autumn  Autumn release  [active]
[exit 0]
```

You add a card to a workstream with `dinah join`, and you take it out again
with `dinah leave`. The card is what you name first, because the card's own
file is what changes:

```console
$ dinah join rel-1 autumn
rel-1  Write the release notes  [Done / ready]  autumn
[exit 0]
$ dinah join rel-2 autumn
rel-2  Draft the changelog  [Intake / ready]  autumn
[exit 0]
```

The workstreams a card belongs to print at the end of its line, and they print
there after every command that draws one. `dinah workstream` with no argument
lists what the workbench carries, with the number of live cards in each:

```console
$ dinah workstream
  Slug    Name            Status  Cards
  ------  --------------  ------  -----
  autumn  Autumn release  active  2
[exit 0]
```

Naming one reads its fields and the cards belonging to it:

```console skip=the member listing orders by the stamp a card arrived in its state under, and the replay runs the whole narrative inside one second, so whether the two cards tie on that stamp and fall back to the creation ordinal is decided by where a second boundary falls
$ dinah workstream get autumn
  Field   Value
  ------  --------------
  slug    autumn
  id      8c3b92a3c21a
  title   Autumn release
  status  active
  cards   2

  Card   Title                    State
  -----  -----------------------  ------
  rel-1  Write the release notes  Done
  rel-2  Draft the changelog      Intake
[exit 0]
```

Taking a card out again names the card first, for the same reason joining it
does:

```console
$ dinah leave rel-2 autumn
rel-2  Draft the changelog  [Intake / ready]
[exit 0]
```

The status is yours to write and Dinah never reads it. It says `active` when
Dinah creates the workstream, and you may put any word you like in its place:

```console
$ dinah workstream set autumn status finished
autumn  Autumn release  [finished]
[exit 0]
```

If you want the workstream out of your listings when the effort is over, run
`dinah archive workstream/autumn`. That works while cards still belong to it,
and those cards keep the membership. `dinah delete workstream/autumn --yes`
destroys it instead, and Dinah refuses that while a live card still belongs to
it. A workstream names its kind in both of those commands, and nothing else
does, so a workstream and a state may share a name without either one hiding
the other.

## Taking things out

`archive` takes a card, a state, or a comment or attachment on one, out of the
listings and keeps its files. `delete` destroys the same things and their
history. Dinah says nothing to you when either one succeeds.

```console
$ dinah archive rel-3
[exit 0]
$ dinah ls done
  Card   Standing  Title
  -----  --------  -----------------------
  rel-1  ready     Write the release notes
[exit 0]
$ dinah delete rel-1/comments/2
dinah.unconfirmed delete destroys history, so it needs --yes
[exit 2]
$ dinah delete rel-1/comments/2 --yes
[exit 0]
```

When you archive a card, Dinah moves it under `archive/cards/` in the workbench
and stops listing it. Archiving a state asks more of you, since Dinah refuses
while any card still stands there and refuses again if the state is the last one
left. `delete` makes you pass `--yes`, because you cannot recover anything you
delete.

## When the files are wrong

You may edit the files by hand, and when you do you can make mistakes in them.
Run `check` to find them. The workbench answering below is not the one you have
been working in. It was damaged on purpose for this example, and its cards name
a state that no longer exists:

```console skip=the transcript answers from a workbench damaged on purpose, which the narrative never builds
$ dinah check
  a card names state 000000000000, which this workbench does not declare (/home/ana/damaged/.dinah/d0e41d414bb5/cards/73ca475d0aaa/card.md)
  the journal puts this card in state fcd0d92e167a, and its frontmatter disagrees (/home/ana/damaged/.dinah/d0e41d414bb5/cards/73ca475d0aaa/card.md)
  a card names state 000000000000, which this workbench does not declare (/home/ana/damaged/.dinah/d0e41d414bb5/cards/9a556a230e09/card.md)
  the journal puts this card in state 003b09ee6e31, and its frontmatter disagrees (/home/ana/damaged/.dinah/d0e41d414bb5/cards/9a556a230e09/card.md)
4 defects.
[exit 2]
```

Dinah names the file to open on every line. Open each one in your editor, fix
it, and run `check` again.

`check` also catches a claim without the substate that implies it, a block with
no reason, a link pointing at no card, a journal whose last line was cut off,
and a directory carrying no anchor file where a comment or an attachment should
be. It only reads and reports, and it changes nothing unless you ask it to.

`check` reports three things that mark an older workbench rather than a mistake,
and you repair each of them with a flag. The workbench answering below is that
older one, kept for this example rather than the workbench you have been
building. Your own `check` still reports nothing at this point. A state written
before slugs existed carries no slug, a comment or an attachment written before
ordinals existed carries no ordinal, and a state that was moved or removed
without an edit to `workbench.md` leaves its identifier stranded in the list:

```console skip=the transcript answers from a legacy workbench, which the narrative never builds
$ dinah check
  aeed974a5f22 carries no creation ordinal, so its position depends on the directory listing (/home/ana/legacy/.dinah/d0e41d414bb5/cards/73ca475d0aaa/comments/aeed974a5f22/comment.md)
  fcd92b769691 carries no creation ordinal, so its position depends on the directory listing (/home/ana/legacy/.dinah/d0e41d414bb5/cards/73ca475d0aaa/attachments/fcd92b769691/attachment.md)
  state 003b09ee6e31 carries no slug, so it is reachable only by its identifier or its quoted title (/home/ana/legacy/.dinah/d0e41d414bb5/states/003b09ee6e31/state.md)
  state 780659205f6b carries no slug, so it is reachable only by its identifier or its quoted title (/home/ana/legacy/.dinah/d0e41d414bb5/states/780659205f6b/state.md)
  state fcd0d92e167a carries no slug, so it is reachable only by its identifier or its quoted title (/home/ana/legacy/.dinah/d0e41d414bb5/states/fcd0d92e167a/state.md)
  the workbench names a state whose directory is not there (000000000000); dinah check --migrate-states removes it from the list (/home/ana/legacy/.dinah/d0e41d414bb5/workbench.md)
6 defects.
[exit 2]
$ dinah check --migrate-slugs
Assigned 3 state slugs.
  Slug    Title
  ------  ------
  intake  Intake
  doing   Doing
  done    Done
  aeed974a5f22 carries no creation ordinal, so its position depends on the directory listing (/home/ana/legacy/.dinah/d0e41d414bb5/cards/73ca475d0aaa/comments/aeed974a5f22/comment.md)
  fcd92b769691 carries no creation ordinal, so its position depends on the directory listing (/home/ana/legacy/.dinah/d0e41d414bb5/cards/73ca475d0aaa/attachments/fcd92b769691/attachment.md)
  the workbench names a state whose directory is not there (000000000000); dinah check --migrate-states removes it from the list (/home/ana/legacy/.dinah/d0e41d414bb5/workbench.md)
3 defects.
[exit 2]
$ dinah check --migrate-ordinals
Stamped 2 creation ordinals.
  the workbench names a state whose directory is not there (000000000000); dinah check --migrate-states removes it from the list (/home/ana/legacy/.dinah/d0e41d414bb5/workbench.md)
1 defect.
[exit 2]
$ dinah check --migrate-states
Removed 1 stranded state from the list.
  000000000000
No structural defects found.
[exit 0]
$ dinah check
No structural defects found.
[exit 0]
```

Dinah prints what each migration wrote and then reports whatever it did not fix.
You keep getting exit code 2 until the workbench is clean. Dinah takes the slugs
from the titles and reads the ordinals back out of each card's journal, so keep
your journals intact. If the workbench matters to you, run any of the migrations
against a copy first.

You use a fourth flag, `--finish`, in a rarer case. If a power cut or a killed
process interrupts Dinah part way through a structural change, Dinah leaves a
lock file behind naming what it was doing, and `check` reports `a structural
action was interrupted here`. `check --finish` reads the journal to decide
whether that action reached its point of record, then completes it or rolls it
back. Run `check` after any hand edit, and before you commit a workbench to
version control.

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

```console stream=out
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

```console stream=err
$ dinah claim rel-9
unknown-card this workbench carries no card rel-9; run `dinah ls` to see the cards this workbench carries
[exit 2]
```

Dinah writes the sentence after the name for a person to read, and it may
translate that sentence. It never translates the name. In bash or zsh, you cut
that first word out:

```console skip=the line carries a pipe and a redirection, which the in-process head cannot honour
$ dinah claim rel-9 2>&1 >/dev/null | cut -d" " -f1
unknown-card
```

That block runs only on POSIX systems. Branch on Dinah's own exit code, and
capture it before a pipe puts another program's status in its place. PowerShell
may also decorate a native command's error stream when it redirects one. Take
the `--json` route above when you want the same behaviour everywhere.

Dinah takes some of those names from the shared rules that every
Dinah-compatible tool follows, and coins the rest itself. Dinah puts a `dinah.`
prefix on the names it coins. Your script tells the two groups apart by matching
on that prefix. `unknown-card` and `at-capacity` come from the shared rules.
`dinah.unknown-key` and `dinah.unconfirmed` are Dinah's own.

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
and never translates them. You get the same bytes from the same command under
any language setting.

```console
$ dinah ls intake --json
{
  "state": "003b09ee6e31",
  "cards": [
    {
      "id": "9a556a230e09",
      "ref": "rel-2",
      "title": "Draft the changelog",
      "state": "003b09ee6e31",
      "state_title": "Intake",
      "substate": "ready",
      "revision": "sha256:433dfb7fa7a8a24d20c91ca5f9a3d9c50796139787358b7bbeaae9a35717db6c"
    }
  ]
}
[exit 0]
```

The `revision` is the card's content as you read it, reduced to a hash. Dinah
measures the stale outcome against it. If your tool reads a card, thinks, and
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

```console
$ dinah --lang hi status
Release 0.2  (/home/ana/release-notes/.dinah/d0e41d414bb5)  [खोज]
ana के रूप में, संचालक: हाँ

  उपनाम   नाम     प्रकार  कार्ड  कार्य          स्वामी
  ------  ------  -----  ----  ------------  -----
  intake  Intake  आवक    1     कोई कार्य नहीं  एजेंट
  doing   Doing   काम    0/1   लिया जाता है   एजेंट
  done    Done    समाप्त  1     कोई कार्य नहीं  एजेंट
[exit 0]
```

You wrote the titles, and Dinah leaves them as you wrote them. It leaves the
slugs derived from them alone as well. To see which languages your build
carries, ask:

```console
$ dinah version --catalogs
dinah 0.1.0
conforms to dinah-core/0.5
storage format 1

Catalogs:
  Language  Translated
  --------  ----------
  en        622/622
  af        0/622
  cs        0/622
  de        622/622
  es        0/622
  fil       0/622
  hi        622/622
  id        0/622
[exit 0]
```

If you ask for a language whose catalog has no entries, Dinah falls back to
English message by message, and you see no change yet.

## Open a card in your editor

`edit` accepts the same references `path` does and hands the file to your
editor. Dinah looks at `DINAH_EDITOR` first, then your own configuration file,
then `VISUAL`, then `EDITOR`, and falls back to a platform default. Dinah puts
its own variable on top because you share `EDITOR` with every other tool you
run. If you want git and Dinah to open different editors, you have nowhere else
to say so.

Set `DINAH_EDITOR` the way your shell sets any variable, as the scripting
section above shows. The run below points it at `cat`, and you see the file
printed instead of a window opening:

```console env=DINAH_EDITOR=cat
$ dinah edit rel-1
---
title: Write the release notes
number: 1
state: fcd0d92e167a
substate: ready
workstreams:
  - 8c3b92a3c21a
---
[exit 0]
```

Dinah runs that value as a program name, so give it the name of an editor rather
than a command line with flags in it:

```console
$ dinah config set editor nano
[exit 0]
$ dinah config get editor
nano
[exit 0]
```

If you set none of those and Dinah finds no fallback editor on the machine,
`edit` fails with `dinah.no-editor`.

## Work on a workbench you are not standing in

Dinah walks up from the working directory to find its workbench. You can stand
anywhere inside one. To see what that walk reaches from where you are standing
now, run `workbenches`. Dinah answers you with rows rather than an error, and it
tells you plainly when it reached none:

```console
$ dinah workbenches
  Workbench    Slug  Path
  -----------  ----  -------------------------------------------
  Release 0.2  rel   /home/ana/release-notes/.dinah/d0e41d414bb5
[exit 0]
$ cd ..
$ dinah workbenches
no workbench is reachable from here
[exit 0]
```

Dinah only climbs. It reaches a workbench above you but never one in a directory
beside you. From outside, name the workbench you want with `--workbench`, or set
`DINAH_WORKBENCH`. Point either one at the workbench itself, the
`.dinah/<identifier>` directory holding `workbench.md`, rather than at the
project directory above it:

```console
$ dinah --workbench release-notes/.dinah/d0e41d414bb5 status
Release 0.2  (/home/ana/release-notes/.dinah/d0e41d414bb5)  [flag]
acting as ana, operator: yes

  Slug    Name    Kind    Cards  Work        Owner
  ------  ------  ------  -----  ----------  -----
  intake  Intake  intake  1      none taken  agent
  doing   Doing   work    0/1    taken       agent
  done    Done    done    1      none taken  agent
[exit 0]
```

If Dinah reaches no workbench at all, it fails with `dinah.no-workbench-found`
and names both the directory the climb started from and the home directory it
fell back to. You are standing in ana's home now, and it holds no workbench of
its own yet:

```console
$ dinah status
dinah.no-workbench-found no workbench was found walking up from /home/ana, or in the user base at /home/ana/.dinah; run `dinah init` here to create one, or pass --workbench <dir> to point at one that exists
[exit 2]
```

You can keep workbenches of your own in your home directory, in the `.dinah`
directory beside your settings, and Dinah falls back to those when the climb
finds nothing. If you keep two or more of them there, Dinah will not choose
between them for you. The two below are ana's own, made in her home directory
and titled by hand afterwards. You will see the same thing only if you have made
more than one workbench there yourself:

```console skip=the workbench listing is ordered by an identifier the tool mints at random
$ dinah init --slug alp --operator ana
Workbench created at /home/ana/.dinah/cd20d36303bc.
[exit 0]
$ dinah init --slug bet --operator ana
Workbench created at /home/ana/.dinah/2ae23a55a39c.
[exit 0]
$ dinah workbenches
  Workbench     Slug  Path
  ------------  ----  -----------------------------
  Household     bet   /home/ana/.dinah/2ae23a55a39c
  Reading list  alp   /home/ana/.dinah/cd20d36303bc
[exit 0]
$ dinah status
dinah.ambiguous-workbench more than one workbench is reachable from /home/ana/.dinah
  Workbench     Slug  Path
  ------------  ----  -----------------------------
  Household     bet   /home/ana/.dinah/2ae23a55a39c
  Reading list  alp   /home/ana/.dinah/cd20d36303bc
choose one with --workbench <dir>, or run from inside it
[exit 2]
```

If you run `show` with no argument from a place where more than one workbench is
reachable, Dinah prints that same listing. Inside a resolved workbench a bare
`show` behaves differently and takes the ordinary route for a card reference it
cannot find. Here you asked Dinah to show you something and it cannot tell which
workbench you meant, so it shows you the choices instead of refusing, and it
exits 0:

```console skip=the workbench listing is ordered by an identifier the tool mints at random
$ dinah show
  Workbench     Slug  Path
  ------------  ----  -----------------------------
  Household     bet   /home/ana/.dinah/2ae23a55a39c
  Reading list  alp   /home/ana/.dinah/cd20d36303bc
```

You move that fallback directory by setting `DINAH_HOME`. If you point it
somewhere else, Dinah reads your settings file and any workbenches under it from
there instead. Set it when you want to work against a scratch setup without
touching your own.

## Hand a workbench to somebody else

`export` prints the whole workbench definition in the shared exchange format,
and another program built to the same rules can read what it prints:

```console
$ cd release-notes
$ dinah export
{
  "instructions": "Every card on this workbench ends with a line in the changelog.\n",
  "profile": "dinah-core/0.5",
  "states": [
    {
      "id": "003b09ee6e31",
      "kind": "intake",
      "slug": "intake",
      "title": "Intake"
    },
    {
      "capacity": 1,
      "id": "780659205f6b",
      "instructions": "Work the card until it is finished or until something stops you.\nLeave a comment saying what you did before you carry it on.\n",
      "kind": "work",
      "slug": "doing",
      "title": "Doing"
    },
    {
      "id": "fcd0d92e167a",
      "kind": "done",
      "slug": "done",
      "title": "Done"
    }
  ],
  "title": "Release 0.2"
}
[exit 0]
```

The printed definition carries the workbench's declared level sets, and any
other block its frontmatter holds, as JSON of the same shape: a nested mapping
prints as an object, a list prints as an array, and a level entry carrying a
hint prints as a one-member object inside that array. A workbench you start
from the result gets those blocks back in its own frontmatter, so nothing you
declared is left behind by the trip through the exchange format.

`extract` writes that same definition to a directory as a template you can use
again. It carries the flow and the instructions, and none of the cards. You
start a new workbench from a template with `init --from`:

```console
$ dinah extract ../release-template
Definition written to ../release-template.
[exit 0]
$ cd ..
$ mkdir release-0.3
$ cd release-0.3
$ dinah init --from ../release-template --slug rel3 --operator ana
Workbench created at /home/ana/release-0.3/.dinah/e65a73e02874.
[exit 0]
$ dinah states
  Slug    Name    Kind    Cards  Work        Owner
  ------  ------  ------  -----  ----------  -----
  intake  Intake  intake  0      none taken  agent
  doing   Doing   work    0/1    taken       agent
  done    Done    done    0      none taken  agent
[exit 0]
$ dinah instructions doing

Instructions, this workbench:
Every card on this workbench ends with a line in the changelog.

Instructions, this state:
Work the card until it is finished or until something stops you.
Leave a comment saying what you did before you carry it on.
[exit 0]
```

The template carries the state identifiers, the slugs, and both layers of
instructions. A workbench you start from it names its states exactly as the old
one does and serves the same standing text. That last `cd` puts you back in the
workbench this guide started in, and the commands below expect you to run them
there.

```console
$ cd ../release-notes
```

## The guides that ship inside Dinah

Dinah carries its own guides, and you can read them with no network and no
repository checkout:

```console
$ dinah guide
The guides stand in the order Dinah recommends reading them.

  Topic             Title
  ----------------  ------------------------------------
  first-session     Your first session at a workbench
  getting-started   Getting started
  verbs             The five verbs
  principles        Why a workbench has the rules it has
  references        References
  query             Asking questions of a workbench
  workbench-layout  What a workbench looks like on disk
  mcp               Working over MCP
[exit 0]
```

Read `dinah guide workbench-layout` before you start editing files by hand,
because it maps the whole directory for you.

Read `dinah guide mcp` before you point an agent at the workbench, because
it teaches the machine surface the way an agent reads it rather than the way
a person types commands.

## Point an agent at the workbench

`dinah mcp` serves the workbench over MCP on its standard input and output, so
an AI colleague can work the same board you do. Configure it in your MCP client
as the command `dinah mcp`, and either run it from somewhere inside the
workbench or point `DINAH_WORKBENCH` at the `.dinah/<identifier>` directory, the
same path `--workbench` takes. Dinah hands the client the rules for working this
workbench and twenty-one tools against its twenty-nine commands. Every command
that files, moves, or reads a card is there. Seven of the eight that are
missing only make sense at a shell: `init`, `config`, `path`, `edit`,
`extract`, `workbenches`, and `mcp` itself. The eighth is `guide`, and the
client reads it as a resource rather than calling it as a tool. Your AI
colleague claims, moves, releases, and blocks under the same rules and leaves
the same journal entries you do.

Give your AI colleague an actor name of its own through `DINAH_ACTOR`, so the
record shows who did what.

## Where to look next

`dinah help <command>` lists a command's arguments and every reason it can say
no, in the order Dinah makes the checks. Read that order when you are working
out which of two possible errors you are looking at.

```console
$ dinah help claim
claim <card> [--expires <duration>]

Take up a ready card

What you may write:
  As you write it         What it is
  ----------------------  -------------------------------------------------------------------------------------------
  <card>                  the card you are taking up
  [--expires <duration>]  how long your claim holds before it goes stale, written as a number and a unit: 30m, 2h, 7d

What can go wrong, in the order each is checked:
  Order  What can go wrong                                          Refusal
  -----  ---------------------------------------------------------  -------------------
  1      the workbench declares a major number the tool implements  unsupported-version
  2      the workbench designates an operator                       no-operator
  3      the card exists                                            unknown-card
  4      the request names an owner                                 no-owner
  5      the owner named as holder is the owner asking              not-requester
  6      the card's substate is not `blocked`                       blocked
  7      the card's substate is not `active`                        held

Exit codes: 0 ok, 2 refused, 3 stale, 4 unreachable.
[exit 0]
```
