---
format: 3
profile: dinah-core/0.12
title: Dinah development
operator: paul
levels:
  severity: [trivial, minor, major, critical]
  priority: [later, soon, next, now]
  tier: [minimal, workhorse, frontier]
groups:
  DESIGN: [2f6c18c9f5d0, ca3badf49985, 0d86ad99cdbc, 5729d4578008]
  BUILD: [0789fd2dbefd, 4fda9c9ca779, 4b38abe7ebd5, ee29487fad76]
  VERIFY: [c9428b3bc921, 6c5b9d6f4414, b69abf918c42]
columns:
  - 5ea2db0272fc   # Intake
  - a2eb2436b77d   # Triage
  - 2f6c18c9f5d0   # Design Queue
  - ca3badf49985   # Spec
  - 0d86ad99cdbc   # Agent Design Review
  - 5729d4578008   # Operator Design Review
  - 0789fd2dbefd   # Build Queue
  - 4fda9c9ca779   # Implement
  - 4b38abe7ebd5   # Agent Code Review
  - ee29487fad76   # Operator Code Review
  - c9428b3bc921   # Test
  - 6c5b9d6f4414   # Merge
  - b69abf918c42   # Acceptance
  - aa6cd1c6ae5f   # Done
slug: dinah
---
This workbench holds the development of Dinah itself, and it is the arbiter of live work for this project. A reader who wants to know where a piece of work actually stands reads it here, with `dinah next`, `dinah ls` and `dinah show <card>`, and moves work with `dinah move`. Cards are filed here and nowhere else.

It continues the hosted Andoneer board whose identifier is `149f228d48c3`, and it carries that board's fourteen states, in the same order, under the same column identifiers. That board is now the archive of everything that happened before the cutover: its cards, their comments and their histories stay readable there and nothing new is filed on it. History did not cross, because every verb here stamps its own clock and a replayed journal would read as true without being true.

Dinah is the open-source, single-seat command-line tool spun out of Andoneer: a reference implementation of a shared coordination contract, covering the workbench definition, its states and transitions, the instructions served at each state, the claim, move, release and block verbs, and the working agreement. The name is a recursive acronym, "Dinah Is Not A Harness", and the expansion is the scope statement: Dinah coordinates work, and the harness around the agents is somebody else's job. Dinah.Team is the hosted, multi-seat product built on Dinah's own library rather than being a second implementation of the contract. The command-line tool stays deliberately tiny, with no board interface of its own. The first external tester is a colleague whose eleven-step resolution workflow becomes the first workbench definition that is not this one, so nothing software-specific goes into the contract's shared core and domain text lives in layers a non-software workbench can decline.

## How work moves here

This workbench runs one route and every card walks all of it: Intake, Triage, Design Queue, Spec, Agent Design Review, Operator Design Review, Build Queue, Implement, Agent Code Review, Operator Code Review, Test, Merge, Acceptance, Done. There is no second path and no routing field. A card that needs a shorter road is a card whose route this workbench has not yet been asked to grow.

**Cards keep moving, and keeping them moving is the job of whoever moved them.** Nothing here watches for a card arriving somewhere, so a card that lands in a column stays there until somebody runs the work waiting for it. After every move you make, read where the card landed and carry it on. The same applies to a card the operator unblocks or moves while you are working elsewhere: his move is not a notification, so read the workbench again rather than trusting the picture you had of it.

**Three columns stop a card, and they are the ones where the operator acts.** Acceptance is owned by him outright, so nobody else moves a card out of it. His two review stations are not owned, on his ruling of 2026-09-14, so a clean card runs past them and only a card carrying a pending item naming the station stops there. A card that reaches any of the three is finished with you: say in your reply that it is waiting there, and stop.

**A move carries no claim with it.** Moving a card into a column leaves it standing there ready, so when you are going to do that column's work yourself, claim the card before your first edit and hold the claim until you move it on. Working a card the workbench shows as ready is the failure the working agreement exists to prevent. A claim is released when your work stops for any reason other than moving the card on, because a card left claimed and idle reads as though somebody were producing output for it while nothing is happening at all.

**Filing a card is not moving it.** A card sitting in Intake is the operator's to work through.

## Where an item is filed, and which way its column holds

An item names exactly one column, and that column decides where the card stops. The direction is not a preference.

**An item is filed against the column that settles it, and that column holds on the way out.** A decision the implementer takes names Implement. A question the reviewer settles names the review station. A question only the operator can rule on names the operator station that answers it.

**An item that must already be settled before a station is reached names that station, and that station holds on the way in.** An acceptance criterion is the case this workbench runs: a criterion names Merge, on the operator's ruling of 2026-09-01, because Test verifies criteria and holding one at Test would require it to be verified before the column that verifies it.

An exit hold reads only the column the card is leaving, so an item naming a column the card has already passed holds nothing at all. That is why a question raised at Test, Merge or Acceptance blocks the card where it stands rather than being filed against a station behind it, and it is the single most common way a stop somebody meant to create silently fails to exist.

The hold reads the card's items and never the actor, so it refuses the operator exactly as it refuses anybody else. That is what gives his two review stations a stop now that they are not owned. He can lift his own stop with an override, which only he may use and which the move records.

Seven columns hold on the way out: Spec, Agent Design Review, Operator Design Review, Implement, Agent Code Review, Operator Code Review and Test. Merge holds on the way in. The other six hold neither way.

**An item stamped `--owner operator` can be closed only by the operator.** Anybody else is refused by name. An unstamped item reads as his, so a question your own next stage will answer is stamped `--owner holder` explicitly.

## Fix it on the card that found it

The operator ruled this on 2026-09-10, after watching the intake queue grow while the work was going well. Over one stretch this project filed about twenty-one new cards and merged nine, so every session of good work left him further from finishing.

So the default is to fix what you find, on the card you are already holding, and the bar for filing is this: **file a card only if somebody would pick it up on its own.** A finding that will only ever be fixed as part of the next card touching that file is not a card. It is a note, and it belongs on the card that found it.

Three routes, in order of preference. Fix it in the work you are already doing, and say in your handoff that you did and what it was. Where a stage may not make the change, which is a review stage meeting a defect in a contract, record it on the card so the next stage fixes it. File a new card last, and when you do, say in it why it could not be cleared where it was found.

**One card per class of defect, with its instances, rather than one card per sighting.** A class with seven cards is not seven pieces of work. When you meet a fresh sighting of something already filed, add it to that card with its location and its evidence.

This is not permission to widen the card you are holding past its contract, and it is not permission to leave something unrecorded because filing feels expensive. Silence is worse than either a fix or a card.

## Working safely against the operator's own data

All work on this repository happens inside a git worktree under `C:\dinah-scratch\`, in a directory named for the card you are working. Never work against the checkout at `C:\Users\paul\source\repos\dinah`, and never put a worktree under `.claude/worktrees/` inside it.

The reason is how Dinah finds a workbench. Discovery climbs from the current directory to the drive root, so a worktree anywhere inside the repository sits below the repository's own workbench, and a worktree anywhere inside the operator's profile reaches the live workbenches in his home. `DINAH_HOME` does not bound that walk. A command run from such a worktree therefore operates on his real data while appearing to be isolated.

Create the worktree yourself rather than letting a harness create one where it prefers:

```
git -C C:/Users/paul/source/repos/dinah worktree add --detach C:/dinah-scratch/<card>-<stage>/wt origin/<branch>
```

**Every command carries its own directory.** The shell's working directory does not persist between calls and resets to the operator's checkout, so a command that does not say where it runs runs there. Write `cd <your worktree> &&` or `git -C <your worktree>` on every call. Keep your binaries, fixtures and temporary directories under your own card's directory, and when you finish delete that directory and nothing above it, because the scratch area is shared and the directory above yours holds another card's work.

A worktree cannot remove itself, so remove yours from a different one with `git -C <another linked worktree> worktree remove <the one you finished with>`. If you have already deleted the directory, `git worktree prune` clears its registration.

Report the directory you actually worked in, not the canonical repository path. A handoff naming the operator's checkout reads as a breach and costs whoever reads it a check to disprove.

## Disciplines this project has paid for

**A test that cannot fail is worse than no test**, because it buys confidence it has not earned. Arm every test by breaking the behaviour it guards, watching it go red, restoring from a byte-identical copy, and watching it go green again, then say what the red run said. Break the behaviour rather than deleting the assertion. Confirm the plant compiled and the run executed, because a break that fails to compile produces no output and no output looks exactly like everything passing.

**Where a check sweeps a set, assert how big the set was.** A sweep that reads nothing reports success, and it reads exactly like a sweep that found nothing wrong. Give each half of a two-part sweep its own count.

**Where a document states a fact about the code, check it against the code** rather than against another sentence. Counts and memberships have shipped wrong here more than once, each time because somebody carried a number forward from prose. Prefer committing a derivation that holds the document to the code over running one and throwing it away.

**A claim about where something happens is checked by tracing the call path**, not by reading the function. One card spent three review rounds on a claim that reading the function supported every time and that tracing found wrong for three of six callers.

**Searching a document for a phrase finds copies of the phrase, not copies of the claim.** Read for meaning rather than searching for wording, and after you finish editing read your own new text as a stranger.

**Do not close a paragraph with a sentence nobody asked for.** Three consecutive rounds on one card each repaired a false claim and each shipped a fresh one, every time as a summarising flourish rounding off the paragraph just corrected. Write the minimum that is true.

**A correction that narrows a premise must be read against the conclusion drawn from it**, because the sentence that was true only under the wider premise is usually left standing underneath it.

**A guard widened by example fits only its examples.** When you find yourself writing a third variant of one regular expression, parse the construct instead, or delete the guard and say plainly in the code what is not guarded.

**Where a guard cannot see something, say so where somebody meets it**, with the reproduction that demonstrates it, rather than closing the card on a promise.

**A refusal that any value satisfies is not a refusal**, and **a criterion asserting that something is refused passes against code that refuses everything**, so pin the accepting case beside the refusing one.

**Green on your machine is not green.** Two branches can each pass their own suite and fail together. Merge the current trunk into your branch, run the suite on the merged result, and read the pull request's checks on every platform. A merge can break a build with no conflict marker at all, so build the merged result rather than reading the merge.

**Check that the branch carries the trunk.** Mergeable and green prove nothing if the branch predates what has since landed.

**Do not build a guard that fires falsely in a language nobody here can adjudicate**, and measure the rate of false firing before proposing a guard or declining one.

## Standing documents

The workbench carries its standing documents as attachments, listed by `dinah attachments workbench`: the prose standard that governs every prose surface, the Go style standard that governs every Go file, and the counterexample corpus of idiom pairs this project has already paid for. Each is named in the column bodies that require it.

## Link kinds

This workbench declares five link kinds and uses the Andoneer board's spellings verbatim: `blocks`, `relates_to`, `supersedes`, `parked_behind` and `spawned_from`. The kind carries no behaviour on either side, so the spellings are this workbench's vocabulary rather than the tool's.

Five rather than the four this card's own decision enumerated. That decision recorded the principle, which is to adopt the board's spellings verbatim, and undercounted the set: `spawned_from` was the second most used kind on the board, on 71 of its 210 link rows, and dropping it would have thrown away every record of one card having been split off another. Of the five, four carry live links after the cutover and `supersedes` carries none, because all twelve of its links had an end on a card that did not cross.

## Publishing

A card lands on the trunk by having its pull request merged, not by pushing to the trunk, which is refused. Every pull request is authored as the project's machine identity rather than as the operator's own account, because a pull request's author cannot approve it and one opened under his account silently removes the review this project depends on. Only the create call needs that identity; pushes and reads stay on the default authentication. Never copy the token itself into a file, a note, or any output. Commit authorship is not part of that constraint and needs no correction.

Re-running a release mints a tag and publishes, so it is the operator's call and never a stage's.

## Two things about this repository that cost a card each

The test binary clears the environment it must not inherit, so do not tell anybody to unset a variable before running the tests and do not read a failure as environmental without proving it. The operator keeps an editor variable set because he uses it, and the suite passes with it set.

One fixture still keys on source line numbers and shifts when a diff adds or removes lines above it, with nothing announcing it. After any merge resolution, recompute rather than reconcile, and open each anchored line and read it rather than checking the arithmetic.

## What this workbench does not carry

The retired checklist states. Dinah declares four item states, pending, resolved, verified and failed, and has no state meaning that an item no longer applies. A card crossing the cutover with such an item carries it in its body under the literal heading `## Retired checklist items (no Dinah state for these yet)`, and one anchored search over the card anchors is a complete census of them at any later date. dinah-472 is the card that adds the missing state, and the census returns to zero when it lands.

A card's branch name. A card's fields are title, body, severity, priority and tier, so the branch lives in the card body under the literal heading `## Branch`, on a line of its own, and every stage reads it from there.

Card journals and comments from before the cutover. They stay on the Andoneer board, which is the archive.
