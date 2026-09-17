# Dinah

Development of Dinah runs on Dinah's own workbench, which lives in this
repository at `.dinah/01a09db38b9f78ebafed830e644adf4a`. The board is the
authority for where a card stands, what the station you are working expects,
and what the workbench forbids. Read it rather than relying on a handoff
document or on what a previous session remembered.

```
dinah show <card> --fields card,body,links,attachments   # never a bare show
dinah instructions <column>                              # what this station expects
```

`CLAUDE.md` in this directory carries the two GitHub identities and when each
one is used.

## Delegation is prescribed, not optional

This board runs a fourteen-station route, and each station's work belongs to
an agent standing at that station. The session that holds the board
orchestrates and does not do the station work itself.

**Dispatch the station's profile without asking.** A card standing ready at a
station the board delegates is a card waiting for a dispatch, and waiting for
permission to make one is what stops the route. The profiles are defined in
`.devin/agents/`, one per delegated station, and each one carries that
station's own instructions.

Six stations are delegated: Spec, Agent Design Review, Implement, Agent Code
Review, Test and Merge. Each has two profiles, one per rung.

## The card's tier picks the profile, not the station

**Read `dinah get <card> tier` before every dispatch, and send the profile at
that rung.** A card declares what it needs, and cards differ, so the model is
the card's business rather than an opinion about how demanding a station is.

| The card's tier | Profile | Declares |
|---|---|---|
| `frontier` | `card-<station>-frontier` | `claude-opus-5` |
| `workhorse` or `minimal` | `card-<station>-workhorse` | `claude-sonnet-5` |
| `apex` | none yet: stop and ask Paul | |
| none declared | `card-<station>-workhorse` | `claude-sonnet-5` |

A minimal card goes to the workhorse profile because the gate compares a floor
rather than a match, so an over-qualified claimant is admitted, and the saving
below sonnet is not worth a third rung. No profile pins an apex model, so an
apex card stops here rather than being worked by an agent that would have to
misdeclare itself to claim it.

**Never solve a tier refusal by changing what an agent declares.** The three
refusals name three different repairs. `dinah.undeclared-model` means the
declaration was dropped. `dinah.unlisted-model` means the model is absent from
the tiers table in `workbench.md`, which only Paul writes, so it is his ruling.
`dinah.below-tier` means the card wants a higher rung, so dispatch the frontier
profile instead. The journal records what an agent declares, and a declaration
naming a model it is not running is a false provenance record that the gate
will then wave through.

Dispatch one agent per station and read where the card landed afterwards.
Nothing on this board watches for a card arriving somewhere, so carrying it on
is the job of whoever moved it. Never spawn a background child that waits on
itself, and never run a retry on a timer.

## Where the route stops for the operator

Three stations are Paul's. Acceptance is his outright, and nobody else moves a
card out of it. His two review stations are not owned, so **a clean card runs
straight past them**, and one stops there only when a pending item names that
station or when something genuinely needs his ruling.

So the route runs unattended from Spec to Acceptance. Stop and tell him when a
card reaches Acceptance, when an item is waiting for an answer only he can
give, or when a station blocks the card in place.

A question the work can survive is filed as an item against the station that
answers it, and the work carries on. A question the work cannot survive blocks
the card where it stands. Subagents cannot ask him anything directly, which is
why both routes go through the board.

## Reading the board cheaply

A bare `dinah show <card>` serves every comment and every checklist item, and
on a card in flight that is 150KB to 570KB where the reference, body, links and
attachment listing come to about 3KB. `C:\Users\paul\Documents\dinah-token-discipline.md`
is the measured brief. Name the members you need, take the contract from the
spec attachment by path, and pull one comment rather than the whole thread.

## Working safely against live data

Work in a git worktree under `C:\dinah-scratch\`, never in this checkout and
never under `.claude/worktrees/`. Workbench discovery climbs to the drive root,
so a worktree inside the repository sits below the repository's own workbench
and a worktree under Paul's profile reaches the live workbenches in his home.
`DINAH_HOME` does not bound that walk.

Every command carries its own directory, because the shell's working directory
resets between calls and defaults to this checkout.

`.devin/config.json` denies the acts the workbench instructions forbid. Those
rules catch the plain spelling of each command and cannot catch every
spelling, so they are a floor rather than the whole guard. The prohibitions in
the workbench instructions still bind you where a rule does not reach.
