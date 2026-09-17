# Station bindings, common to every card agent on this board

This file is reference material for the station profiles in `agents/` beside
it. It sits outside that directory on purpose: every markdown file under
`.devin/agents/` registers as a subagent profile named after the file, so a
shared document kept in there would register as a profile nobody meant to
create.

Each profile names this file and repeats the bindings it needs, because a
subagent inherits no context from the session that dispatched it.

## The workbench

The board is the workbench in this repository at
`C:\Users\paul\source\repos\dinah\.dinah\01a09db38b9f78ebafed830e644adf4a`.
The `dinah` command finds it by climbing from the current directory, so any
`dinah` call made from inside the checkout reaches it.

**Every mutating `dinah` call carries its own identity, inline, on the
invocation itself.** The shell's environment does not survive between calls any
more than its working directory does, so an `export` at the start of your work
reaches your first call and nothing after it. A call that loses the identity
does not fail. It is recorded against the workbench's operator, so the board
ends up saying Paul wrote your findings, and `--actor paul` is reserved for
writes he actually delegated. This has already happened once, on this card.

Write it out every time:

```
DINAH_PROVIDER=<provider> DINAH_MODEL=<model> \
  DINAH_HARNESS=devin dinah --actor claude <verb> ...
```

**Your profile states the exact provider and model strings to use. Copy them
and compose nothing.** Do not infer a spelling, do not read one off the card's
history, and do not carry one over from an example: the journal records what
you declare, and a declaration that names a model you are not running is a
false provenance record that the tier gate will then let through. If your
profile does not state the strings, stop and say so rather than guessing.

`--actor` fixes the authorship. `DINAH_MODEL` stops the refusal
`dinah.undeclared-model`.

A claim is also refused `dinah.unlisted-model` when the model you declare is
absent from the tiers table in the workbench's own `workbench.md`, and a card
declares the tier it requires. That refusal is a real stop and not a hurdle:
the answer is a ruling from Paul about the table or about which profile works
the card, never a different declaration.

Read-only calls need none of this.

**Check your own authorship before you hand off.** `dinah show <card>/comments/<n>`
prints an `author:` line. Read it on the comments you posted, and say in your
handoff if any of them carries the wrong name, because the next reader cannot
tell a misattributed finding from one Paul actually made.

## Read the station before you work it

```
dinah instructions <column>
```

That text is the contract for the stage, and it outranks anything a handoff
comment or a dispatch prompt told you. Where the two disagree, the column is
right and the other one is stale.

## Claim, then work, then hand off, then move

Claim the card before your first edit, because working a card the board shows
as ready is the failure the working agreement exists to prevent. Hold the
claim until you move the card on. Release it if your work stops for any other
reason.

A move carries no note, so the handoff comment is posted immediately before
the move with nothing between the two. A move does carry the claim.

Pass prose through standard input, because a note with spaces is refused:

```
dinah comment <card> - <<'TEXT'
...
TEXT
```

## Read the board cheaply

Never a bare `dinah show <card>`. It serves every comment and every checklist
item, which is 150KB to 570KB on a card in flight against about 3KB for what
you need. Open with `--fields card,body,links,attachments`, take the contract
from the spec attachment by the path that answer gives you, and pull one
comment at a time with `dinah show <card>/comments/<n>`. Leave the checklist
alone unless an item needs work, and grep it rather than reading it when it
does.

## Items

An item is filed against the column that settles it, and that column holds on
the way out. Your own decisions name your own station. A question only Paul can
rule on names the operator station that answers it and carries
`--owner operator`. A question your own next stage will answer carries
`--owner holder` explicitly, because an unstamped item reads as his.

You cannot ask Paul anything directly. A question the work survives becomes an
item and the work carries on. A question the work cannot survive blocks the
card in place with `dinah block <card> "<the question>" --kind operator-ruling`.

## The tree

Work in a git worktree under `C:\dinah-scratch\`, in a directory named for the
card and the stage. Never work in the checkout at
`C:\Users\paul\source\repos\dinah`, and never put a worktree under
`.claude/worktrees/` inside it. Workbench discovery climbs to the drive root,
so a worktree in either place reaches Paul's live data while appearing
isolated.

```
git -C C:/Users/paul/source/repos/dinah worktree add --detach C:/dinah-scratch/<card>-<stage>/wt origin/<branch>
```

Reuse a worktree that already holds unpushed commits rather than rebuilding
one, because a fresh tree from `origin/<branch>` drops them.

The shell's working directory resets between calls and defaults to the
checkout, so every command carries its own directory. Write `git -C <tree>` on
the invocation itself.

Never `git stash`. Never force-push. Never push the trunk. End every commit
message with `Co-Authored-By: Claude Code <noreply@anthropic.com>`.

## What never happens

Never run `dinah init` in a directory you did not create. Never run
`dinah check` with a `--migrate-*` or `--renumber` flag against a live
workbench; copy it first. Never operate on anything under
`C:\Users\paul\.dinah`, under OneDrive, or on any customer store. Never run
`scripts/verb_selection.py`, which bills about 3.3 million input tokens per
run.
