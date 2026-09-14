# Your first session at a workbench

Somebody has pointed you at a workbench and told you nothing else. This guide
runs in the order a session runs, so you can read it from the top and act as
you go. Each section leaves you where the next one begins, and if you stop
halfway you have still done the first half correctly.

## Find out where you are

Run `dinah status`. One command answers four questions: which workbench
answered you and where it sits on disk, which actor you are and whether that
actor is the operator, what the flow is and how many cards each column holds,
and which cards you are already holding.

Dinah finds a workbench by walking up from the directory you are standing in,
the way git finds a repository, so where you stand decides which workbench
answers you. Pass `--workbench <dir>` or set `DINAH_WORKBENCH` when you want a
different one.

## Find out whose name you are acting under

Run `dinah whoami`. Dinah prints the actor it will record for you, and whether
that actor is the operator.

Dinah reads that name off a ladder and takes the first rung that answers. The
`--actor <name>` flag names it for one command, the `DINAH_ACTOR` variable
names it for a shell, and `dinah config set actor <name>` writes it down until
you change it. `dinah init --operator <name>` is not a rung of its own: it
wrote that last one for you when Dinah did not already know you. Run `dinah
config` to see every setting beside the source that supplied it, so you can
read which one answered before you write anything.

Set your own name before your first act. Dinah records the actor on every act,
the journal only ever grows, and no later command corrects a name already
written.

Dinah compares the name that ladder answers with against the workbench's
recorded operator, and refuses a named set of acts to anybody else under the
refusal name `not-operator`. It refuses `unblock`, a move out of an
operator-owned column, a move carrying `--override`, and `resolve`, `verify` or
`fail` on an item filed `--owner operator`.

The flag outranks the variable and the file. An invocation carrying `--actor`
and the operator's own name is treated as the operator whatever `DINAH_ACTOR`
holds in the shell that ran it, so a caller who wants past that refusal pays
one flag for it.

A caller who can write the files under `.dinah` pays nothing at all. `dinah
path <reference>` prints the anchor of whatever it names, an item's state is a
line of frontmatter in that anchor, and an editor changes it. Afterwards `dinah
check` reports no structural defect, and the journal carries no record that the
item was settled.

The comparison therefore separates names rather than people. It stops you
acting under somebody else's name by mistake, which is worth having in a tool
that records the actor on every act and corrects none of them afterwards, and
it is not a boundary against doing so on purpose. Do not build anything on it
that needs the stronger reading. `ACTOR-4` in the core profile is a rule
addressed to whoever works a card rather than a refusal Dinah enforces
against a caller who has decided otherwise.

## Read what this workbench asks of you

Every workbench states its own rules, and reading them is part of the work
rather than a courtesy you pay it. Run `dinah columns` for the flow in order,
with each column's slug, name, kind, how many cards it holds, and who owns it.

Run `dinah instructions <card>` for the standing prose of the workbench and of
the column that card sits in, followed by the moves that card may make. Run
`dinah instructions <column>` when you want one column's prose on its own. A
workbench or a column that carries no prose prints none and exits 0, so nothing
printed means nobody has written anything rather than that something is wrong.

## Take a card

You take work here rather than waiting to be given it, so you choose your own
card and nobody assigns you one. `dinah next` shows what each column offers
next, `dinah ls <column>` lists one column in the order its cards arrived, and
`dinah query` answers the questions those two cannot. The language `query`
reads is written down in `dinah guide query`.

Run `dinah claim <card>` to take up a card that is waiting. A claim says you
are working that card now, so you claim it before you start rather than once
you have something to show. A successful claim prints the same
standing prose `dinah instructions` prints, together with the moves the card
may make, and `--quiet` suppresses all of it when you have read it already.

## Read the card before you work it

Run `dinah show <card>` for the card itself and `dinah log <card>` for
everything that has happened to it, oldest first. Whoever wrote the card is
not here to answer questions about it, so what the card says is what you have.

A reference reaches below a card as well, to the card's own text, its history,
its comments, its checklist, and its attachments, and `dinah guide references`
is where those spellings are written down. Dinah refuses a `show` that names a
collection rather than one of its members, so name the member you want.

## Record what you did on the card

Run `dinah comment <card> <text>` to record a comment, and
`dinah attach <ref> <file>` to attach a file. Write down what the next reader
needs. They will read this workbench rather than this session, and you will not
be there to fill in what you left out.

## Move the card, then give it back

Run `dinah move <card> <column>` to carry the card on. The move changes where
the card stands and nothing else, so you are still holding it afterwards.

Run `dinah release <card>` to give the card back, and do it as soon as you stop
working, including after the move that finishes your part. A card you are still
holding tells everybody else that somebody is busy with it. Run `dinah status`
to see what you have left behind.

Run `dinah block <card> <reason>` when the work cannot go on. A block frees the
card and records why, and Dinah refuses `dinah unblock` to any actor but the
operator.

## When Dinah refuses

Every command reports one of four outcomes, and each one asks something
different of you. `ok` means it happened, and the exit code is 0. `refused`
means a rule said no, and the exit code is 2. `stale` means the card moved
between your reading it and your acting, so read it again and retry, and the
exit code is 3. `unreachable` means the question could not be asked at all,
and the exit code is 4.

A command Dinah refuses writes the rule's name as the first field of the line,
so `cut -d' ' -f1` gives you the name to act on.

Run `dinah help` for the list of commands, and `dinah help <command>` for one
command's arguments, what can go wrong with it in the order each thing is
checked, and its exit codes. Writing `dinah <command> --help` prints that same
page.
