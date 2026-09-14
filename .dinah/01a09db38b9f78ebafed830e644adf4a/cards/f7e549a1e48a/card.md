---
title: A claim cannot tell a genuine conflict from the asker's own claim still settling
column: 5ea2db0272fc
state: ready
severity: major
priority: next
tier: frontier
workstreams:
  - e9e7b3a9280b
links:
  - kind: relates_to
    to: fd6970afe3dc
---
This is a correctness problem, not an audit-trail problem, and the difference decides what gets built. An audit-trail framing invites a reporting fix: show afterwards who did what. That is not the defect.

The defect is that a claim which cannot name its holder cannot refuse a second claimant at the moment of asking. Every Claude session on the operator's machine reaches this workbench through one identity, so when a second worker asks for a card the first holds, the board has nothing to compare and nothing to refuse with. Duplicate work is therefore the expected outcome rather than bad luck, and the working agreement's first rule, claim before you produce output, is advisory in this configuration while saying nothing about it.

## What that cost, and why it looked cheap

On 2026-09-05 two sessions implemented dinah-5 at the same time, in different worktrees, pushing to the same branch. The second push was refused as non-fast-forward. Nothing was lost, both versions are in the history, and the survivor was chosen deliberately after being read and re-verified.

Git refused that push. The board did not. So the coordination plane failed silently and the content plane caught it by accident.

That accident is not available in general. A card whose work is a document, a comment, an instructions field or a checklist has no non-fast-forward to save it. The two workers simply both write, and the later write wins with nothing recording that an earlier one existed. On this workbench that is a large fraction of the work.

## There are three sessions, not two

Later the same evening a third card, dinah-381, went through three complete spec-and-review cycles in thirty-five minutes while two sessions each believed the other was not touching it. One was about to dispatch a fourth spec on top of the third and stopped only because its agent claimed the card, saw it had moved past Spec, and released it untouched. The work was a third session's, running eight hours, fast-tracking that card on its own operator's instruction. Nothing was wrong with what it was doing.

Listing the machine's sessions settles what the board cannot: three interactive sessions were live. Every transition any of them writes carries the same actor label.

Three is categorically worse than two rather than merely more. With two workers a claim cannot exclude. With three, no worker can enumerate who might be holding a card, and the only thing that established anything all evening was one session naming a peer and asking it directly. That reaches the workers who already know about each other, which makes the entire coping mechanism a coincidence rather than a mechanism.

## The two dispatches had nothing in common

Worth establishing before anyone reaches for the easy fix. The operator announced dinah-5 to one session and not the other. That session read the card, found it ready and unclaimed, and dispatched. The second reached the same card on its own initiative, having carried it through its earlier reviews, and claimed it believing it was promoting work it already held.

The collision needed no shared trigger, no duplicated announcement and no mistake by the operator. One worker was directed and one was autonomous. A fix aimed at how work gets announced would not have prevented it.

## Evidence that the board cannot answer the question

Both sessions reported the same fact in opposite directions and neither report was supportable. One said the board showed a single claim throughout and blamed dispatch. The other said its own claim was taken and the peer never claimed at all. Those are one observation seen from two sides.

## The larger shape, which identity may be one instance of

The same evening a session moved dinah-374 into Agent Code Review, failed to dispatch a reviewer, and reported the review as underway. The card sat ready and nothing was coming. Another session noticed and asked, because reading could not tell it anything: a card standing ready in a review column looks identical whether a reviewer is a second from claiming it or nobody is coming at all.

That is the same ambiguity one level out. In every one of these cases the board carries a single field where two states need distinguishing. Whoever specs this should decide whether the fix is scoped to identity, or whether identity is the first instance of a general problem: that a position on this board records where a card stands and not whether anything is happening to it.

The counterpart is worth stating. Every session reads the workbench's instructions at the start of its work, and those instructions already say that moving a card is not the same as working it. Being told again more loudly is not the fix.

## Why it matters now rather than later

The operator is moving his own work off Andoneer and onto Dinah. Dinah's claim to be a coordination contract rests on the board being the authority for who is doing what.

## What has to be settled

Whether identity on the board is a property of the token or of the session, and if of the session, what mints one.

Refusing a claim when the asker's identity equals the holder's is not the fix, and this card's own evidence rules it out. The log shows an orchestrating session claiming dinah-5, releasing it seventeen seconds later, and the implementing agent then claiming it: three transitions, one identity, every one legitimate. That is the ordinary handoff shape here, and a refusal rule would break it everywhere.

Whether the board should be able to show concurrent holders at all, rather than a single claimed-by field that can only ever name one.

Whether a card in a flow column should be able to say that nothing is coming for it, distinct from saying it is ready.

Whether a worker can discover the other workers on a board at all. Tonight that took a machine-level session listing, which the board neither provides nor knows about.

## Traps

A test that a claim excludes is satisfied by two different identities contending, which already works. The failing case is two or more workers sharing one identity, and a check that does not construct it proves nothing.

Any test of a fix must say what the board would have done with git taken out of the picture, and must include a card whose work is not a git branch at all. Git's refusal is the only reason the dinah-5 collision cost nothing, a check run against a real repository inherits that protection for free, and the document case has no equivalent.

Do not reach for a process id, a machine name, a working directory or a clock to tell sessions apart unless the thing chosen is documented to be stable and unique for that purpose. The operator's standing rule is that a design must not rest a branch point on undocumented behaviour of an external system.

A fix that only makes the collision visible afterwards is worth having and is not the whole job. The value of a claim is that it stops the second worker before the work rather than explaining the wreck after it.

## Related

dinah-376 asks for a card worked while unclaimed, or worked in a column that takes no work up, to be visible rather than indistinguishable from idleness. Same family, different member: on dinah-5 the card was claimed, correctly, and the claim still did not do its job.

## Specification

## What the code does today

`Card.Holder` (`internal/bench/card.go:40`) is a plain string. `claim()` sets it directly from the request's actor: `card.Holder = req.Actor` (`internal/verb/mutate.go:249`). `req.Actor` comes from `ResolveActorSource` (`internal/bench/config.go:178-188`), which tries a per-invocation flag, then the `DINAH_ACTOR` environment variable, then the `actor` key in the user config, in that order, and stops at the first that answers. None of those three rungs carries anything specific to a process or a session; two Claude sessions started from the same shell profile on the same machine resolve to the same string every time.

`claimableState` (`internal/verb/mutate.go:230-237`) already refuses a claim outright whenever `card.State` is active, whoever asks: "A claim cannot take a card somebody is already working, its own asker included" (comment at `internal/verb/mutate.go:226-229`). So the state machine does not fail to refuse a second claim while a first stands. It refuses every one of them. What it refuses with is `contract.Held` (`internal/contract/contract.go:96`), rendered as "{detail} holds this card; wait for {detail} to release it" (`internal/msg/locales/en.json:1944-1950`), where `{detail}` is `card.Holder`, the same plain string. A caller refused this way cannot tell, from the message, whether the name it is reading is its own claim still settling or a different session that happens to share its label.

`LockRecord` (`internal/bench/lock.go:18-30`) is a separate, narrower mechanism and does not bear on this. It guards a structural filesystem act (archive, restore, delete), is held for milliseconds, and is released before the act completes. It already carries `Actor` and `PID` (`os.Getpid()`, `lock.go:104` and `:119`), but nothing reads or compares the PID field, and the lock has no relationship to a card's claim.

The format's own design document anticipated the situation this card is about: "One seat running many agents is therefore many actors in one workbench" (`docs/design/format.md:1719-1731`). That sentence is the design's stated expectation that concurrent agents on one seat distinguish themselves by setting distinct actor values. Nothing in the resolution chain (`config.go:178-188`) enforces it, and the incident is exactly the case the sentence didn't anticipate: multiple sessions inheriting one operator's environment, and therefore one actor string, with no second value anywhere to tell them apart.

## Restating the defect

This is a correctness problem. The claim exists to stop a second worker before it produces output, and it can only do that if it can tell the second worker apart from the first. Today it cannot, because the only thing a claim carries is a role label (who is acting: an operator, an agent, a bot account) and the incident's two colliding sessions carried the same one. The state machine's own refusal already fires in the case it can detect (a card already active gets no second claim, full stop), but it fires with a message that cannot be read as anything other than "you already have this," even when what actually happened is "someone else who happens to share your label has this."

Whichever of the three cards this card names, the same fact governs all of them and must not be re-argued by whoever implements the winner. **The ordinary claim, release, claim sequence between an orchestrating session and the agent it dispatches must keep working exactly as it does now.** This card's own evidence already shows it running clean: an orchestrating session claims a card, releases it seconds later, and the agent it dispatched claims it in turn, three transitions under one actor, every one legitimate. A design that refuses a second claim merely because the asking actor's label equals the holder's label breaks that pattern on every card that uses it, which by the operator's own account is most of the workbench's daily flow. Any of the three options below has to leave that sequence alone.

## Documented interfaces, and the one that doesn't apply

The operator's standing rule is that a design must not have a branch point resting on undocumented behaviour of an external system. Read literally against this card, that rule does not forbid reading a process id or a hostname; both are documented values, reachable through documented calls (`os.Getpid`, `os.Hostname`), on every platform Go supports. What the rule does forbid is treating either as *unique for as long as this design needs it to be*. A claim is a lease measured in hours (`docs/design/format.md:1753`), and an operating system is free to hand a process id to an unrelated process once the one that held it exits; nothing documents that it will not, on any platform, and several document that it will. A hostname distinguishes machines, not sessions on the same machine, which is exactly the axis the incident collided on. So the branch point this design must not rest on is not "can I read a process id," which is fine, but "will this process id still mean what I minted it to mean three hours from now," which no platform promises.

The route that avoids the question rather than answering it is a value the tool mints itself: a random token drawn from a documented cryptographic source (Go's `crypto/rand`). Its uniqueness is a stated mathematical property of the generator, not an assumption about a foreign system's behaviour, so it sidesteps the reuse question instead of resting on it. Whichever option below is chosen, if it needs a discriminator, this is the shape it should take: minted once per process at start, never reused, never compared against a process id or a hostname for identity.

**When the property repeats or is unavailable.** A cryptographic RNG can be treated as never repeating within this design's lifetime, so "the token collides" is not a case the implementation needs to handle as a live branch; it is a case an implementer notes as negligible and moves on from, rather than one it silently assumes away. `crypto/rand.Read` is documented never to return an error on the platforms Dinah ships for. Its own documentation also states what happens on the one platform where the underlying call can genuinely fail (a pre-3.17 Linux kernel, before `/dev/urandom` is seeded): `Read` crashes the process irrecoverably rather than handing back an error the caller could catch. So a failed mint is not a case this design can refuse a claim on. The process is already gone before any refusal logic would run. The one thing this design can still do is not make a bad situation worse: the code should check the error `Read`'s signature returns and refuse rather than proceed with a blank or zero discriminator if that return ever were non-nil, because a zero value that reads as "matches everything" or "matches nothing" would be exactly the refusal-any-value-satisfies shape the workbench instructions warn against. That check guards a branch the documented behaviour says is unreachable in practice, and no test can exercise it.

## The three separable changes

The card names three, and none of them is the same work as either of the others. These are numbered 1 through 3; the open question below combines them and letters its combinations A through C on purpose, so a reader is never asked to guess which scheme a given "1" belongs to.

### 1. An identity that can tell holders apart

Give each running process a discriminator alongside its actor label, minted once at process start the way the section above describes, and change what a card's holder means from a single string to a pair: the actor (a role, meaningful to a human reading the workbench) and the discriminator (a session, meaningful only to the tool doing the comparing).

**Cost.** A format change: `Card.Holder` alone no longer answers "who holds this," and something (`Card` itself, or a value carried alongside it) needs a new field for the discriminator. The claim event gains the same field, since it has to record what it wrote. Every caller that currently reads or compares `Holder` has to decide which half it means: the actor, still the right thing to show a person, or the pair, the right thing to compare inside the tool. The discriminator has to travel from an orchestrating session to the agent it dispatches, or the ordinary handoff pattern reads as two different holders and breaks the very sequence this card must protect; the natural carrier is an environment variable set once and inherited, the same way `DINAH_ACTOR` already travels by convention, but the concrete mechanism is a question for the spec pass that follows whichever option is chosen here, not for this one.

**Buys.** The record, and any refusal built on it, has something to compare besides a label two sessions can share. This is the necessary ingredient for either of the other two; neither can be built without it.

**Cannot do, alone.** Minting a discriminator that nothing reads changes nothing a user or an operator observes. On its own this option gives a later card something to consume. It does not, by itself, stop a collision or make one attributable after the fact.

**Dependency.** Stands alone as an isolated first step; it is also a strict prerequisite for Change 2 and Change 3, which cannot be built without it.

### 2. A record that can attribute a claim after the fact

Carry the discriminator into every journal event a claim, move, or release writes, not only into the live `Card.Holder`, so a reader piecing together an incident afterward, human or automated, can tell which of several same-labelled sessions produced a given line.

**Cost.** A journal format change (`Event`, `internal/bench/journal.go:17-47`, gains a field). The compatibility fixtures under `internal/bench/testdata/compat/` are frozen per profile revision and exist to catch exactly this kind of change; a new revision has to be cut and its manifest digest re-blessed in the same diff, recaptured with the repository's own capture script rather than by hand, per the workbench's own standing instruction on that directory.

**Buys.** Directly answers the incident's second failure: "two sessions reported the same fact in opposite directions and neither report was supportable." A journal carrying the discriminator settles that by being read, not by trusting whichever session speaks first or loudest.

**Cannot do, alone.** It does not stop a collision from happening. It only makes one legible afterward. A card whose only defect were the reporting gap would be well served by this alone; this card's description is explicit that the reporting gap is not the defect, so Change 2 alone does not close the card as filed.

**Dependency.** Needs Change 1's discriminator to exist first; there is nothing to attribute a claim to otherwise.

### 3. An outright refusal of a real conflict

Change `claimableState`'s own comparison (`internal/verb/mutate.go:234-236`) so that a claim attempt while the card is active reads the standing holder's discriminator against the asker's: a match is read as the asker's own settling claim (the ordinary handoff, admitted exactly as it is admitted today), and a mismatch is read as a real conflict and refused with a message that can finally say so.

This is not a new refusal bolted onto the claim path. The refusal already exists and already fires on every second claim; what changes is what it is allowed to conclude when it fires, and it can only conclude something new once Change 1 gives it a second field to look at.

**Cost.** On top of Change 1: the comparison itself, at the one call site named above, and a rewrite of `refusal.held` / `refusal.held.next` so the message can distinguish the two cases it will now actually distinguish (today's text, "{detail} holds this card," says the same thing whichever case produced it). The same care the incident already forces applies here without exception: the claim-release-claim handoff must go on succeeding, so the match branch has to be exercised by whatever test proves this option ships correctly, not only the mismatch branch.

**Buys.** The one thing Change 1 and Change 2 do not: a chance to stop the second worker before it writes anything, which is the value a claim exists to provide and the value this card's title now describes. It is the only one of the three that would have refused the dinah-5 collision at the moment either agent asked, rather than only explaining it afterward.

**Cannot do.** Nothing named in this card; this is the option that closes the gap the card's title describes. It inherits whatever the discriminator itself cannot do (see the section above): it cannot make a lease-length identity out of a process id, and it inherits `crypto/rand`'s own failure mode, a mint failure crashes the process rather than handing back a value this option could refuse on.

**Dependency.** Cannot stand alone; there is nothing for a refusal to compare without the discriminator Change 1 mints.

## Traps checked against this spec

A refusal that fires whenever the requesting actor's label equals the holder's, full stop, is not a design any of the three changes above describes, and none of them should be implemented that way; the card's own evidence (three transitions, one identity, every one legitimate) rules that shape out directly, and Change 3 above is written to compare the discriminator, never the actor label, for exactly that reason.

A test of whichever option ships has to construct the failing case honestly: two distinguishable discriminators contending for one card, not two identical actor strings, which already works today and proves nothing about this card.

## Out of scope

dinah-376 is a different member of the same family: a card moved past a column in its flow without the work that column expects, and the workbench's record of where a card stands did not say whether anything was happening to it. This card is about a claim's record of who holds it. They share files, since both touch what the journal and the live card record, but they do not share the defect. This spec does not absorb dinah-376's scope: no work here changes how a move is recorded or how a skipped column is detected.

## Open question

Which of the three changes above does this card's implementation cover, and in what combination? The combinations are lettered A through C, deliberately distinct from the numbers above, since a criterion referring to "option 1" without saying which scheme it means is exactly the confusion this lettering exists to prevent.

(A) Change 1 and Change 3 together: mint a session discriminator and use it to refuse a real conflict at claim time. This is the only combination that would have stopped the dinah-5 collision before either agent wrote anything.
(B) Change 1 and Change 2 together: mint the discriminator and carry it into the journal for after-the-fact attribution, without changing what a claim refuses. This settles the next incident's account but not the next collision.
(C) Change 1 alone: mint the discriminator, with neither a new refusal nor a journal change, leaving both for a later card to build on.

Each change's cost, benefit, and dependency are written out above.

## Acceptance criteria

1. A test in `internal/verb` exercises the orchestrator-then-agent handoff, claim under actor A, release, claim again under actor A, immediately following one another, and asserts both claims succeed. This behaviour exists today; the criterion is that it still passes after this card's implementation lands, run with `go test ./internal/verb/...`. A failing run here means the implementation chose the refusal shape the traps section above rules out.
2. Whatever discriminator this card's chosen option mints is produced by a call documented to be safe for this purpose (`crypto/rand`, or an equivalent documented cryptographic source), verified by reading the implementation: no comparison anywhere in the changed code keys on `os.Getpid()`, `os.Hostname()`, or a wall-clock timestamp as a standalone identity value. A failing check here is a diff that introduces such a comparison.
3. `crypto/rand.Read` is documented never to return an error on any platform Dinah ships for, and its own documentation states that the one platform where the underlying call can fail (a pre-3.17 Linux kernel, before `/dev/urandom` is seeded) crashes the process rather than returning control to the caller. There is therefore no reachable path where a claim survives a failed mint to refuse on it, and no test can construct one. This criterion is downgraded to a code-reading check: the diff must not silently discard the error `Read`'s signature returns and proceed with a blank or zero discriminator standing in for a real one. Where the chosen combination is lettered option C (Change 1 alone, no conflict-refusal built on the discriminator yet), even this reading-only check is deferred to the card that builds Change 3, which the spec should say plainly rather than leaving silently unverified.
4. If the chosen combination includes Change 3, lettered option A in the open question, a test constructs two distinguishable discriminators under one shared actor label, has the first claim the card, and asserts the second is refused, with the refusal's message able to be shown to differ from the message a genuinely stale claim, lettered option A's own matching case, would produce. This is the honest version of the trap check above: a test using one discriminator for both sides proves nothing and must not be accepted as satisfying this criterion.
5. If the chosen combination includes Change 2, lettered option A or option B in the open question, a new compatibility fixture revision under `internal/bench/testdata/compat/` carries the new event field, with its manifest digest re-blessed in the same diff, produced with the repository's own capture script; a manually hand-edited fixture fails this criterion even if the digest matches, because the workbench's own standing instruction on that directory requires the script.
