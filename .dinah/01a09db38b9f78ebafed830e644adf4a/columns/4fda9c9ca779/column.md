---
title: Implement
slug: implement
kind: work
reject_to: design-queue
operator_owned: false
gate_items: out
---
Where specifications become code in the Dinah codebase, a single-binary Go command-line tool. The implementer works in an isolated tree on a branch named after the card, self-tests, pushes the branch, opens its pull request, then moves the card on. The trunk does not see this work until Merge.

### What arrives, and the clarity gate

Read the card body, the checklist and the latest move comment before writing code. A card arrives here with a reviewed contract, because every card on this workbench passes through Spec and both design reviews. If that contract is ambiguous, missing or self-contradictory, do not start: push the card back to Design Queue, which this column declares as its reject target, with a note naming the ambiguity. If the ambiguity is a ruling only the operator can give and no work can proceed without it, block in place instead.

Do not reinterpret silently and do not widen the card. A dependency the specification missed becomes a new card, linked with `dinah link <new> <this> relates_to` and joined to the same workstreams. A problem bigger than this card's scope is filed separately rather than fixed in the same diff, and the workbench's own filing rule decides which of the two it is: fix it here and say so, unless somebody would genuinely pick it up alone.

### A question for the operator that the work can survive

Not every ruling has to stop the build. When a question is genuinely the operator's but you can finish the card despite it, meaning you made a defensible call and want it ratified or redirected, file it with `dinah file <card> open_question "<the question>" --owner operator --column operator-code-review`, put your call and its tradeoffs in the note, and carry on. It is answered at Operator Code Review, two stages ahead, and the hold there stops the card because the item names that column. Name it in your handoff note so the reviewer and then the operator see it coming.

A question you cannot build around blocks in place: `dinah block <card> "<the question>" --kind operator-ruling`.

## Delivering an implementation means delivering a test

This is not negotiable and it is not somebody else's stage. A change that ships without a test covering the behaviour it changed is an incomplete implementation. Writing the test is part of the work, not an addition to it.

When an existing test covers the area you touched, extend that test. Create a new file only when the behaviour genuinely has no home in the suite. A suite that grows by accretion becomes slow, then redundant, then skipped, and a suite people skip protects nothing. Name in your handoff which existing test you extended, or say why a new one was necessary.

**Arm the test.** A test that passes proves nothing on its own, because it also passes when the thing it guards is absent. Break what you fixed, watch the test go red, restore from a byte-identical copy, and confirm green again. Report that you did it, and quote what the red run said.

Arm it by breaking the behaviour, not by deleting the check. A guard armed by removing itself proves only that it runs. Plant the wrong value the guard exists to catch, and watch the guard name it. Confirm the plant actually compiled and the run actually executed, because a break that fails to compile produces no output, and no output looks exactly like everything passing.

## Prose ships under the prose standard

Any prose this card produces, meaning documentation changes, user-facing copy, error text with sentences in it, and the handoff note itself, is written to the workbench attachment named "Prose standard". Agent Code Review holds it against that standard's list of tells, so write to it the first time. When the card rewrites existing prose, the standard's hard constraint applies before style does: meaning cannot change, and the negative clause you are tempted to cut is often the requirement.

## Go source is written to the Go style standard

The workbench attachment named "Go style standard" governs every Go file this card touches, and reading it belongs before the code. Two of its rules produce work at this stage. Run `gofmt -l .` before you hand off and fix whatever it names, because the corresponding job on the pull request runs the same command and any file it lists fails the workflow. And search for prior art before writing a new function, then say in your handoff which existing helper you reused or that you searched and none existed. A handoff silent on reuse is incomplete. A new module dependency is named together with the standard-library route you rejected and what made it insufficient.

## The testing boundary, and what belongs to Test

This column runs unit tests and change validation on the code the diff touched. It does not run the regression matrix, which belongs to Test.

Treat that as binding rather than as a preference. Pre-empting Test is how this column burns budget it does not have.

What you run: `go build ./...` and `go vet ./...`, both non-negotiable and both cheap; `go test` on each package the diff changed; the arming proof on each new or extended test; and any local smoke the diff's surface demands, which for a command-surface change means running the affected command against a throwaway workbench. A schema change is verified on a fresh store and on a store in the pre-migration shape, with data preserved.

What you do not run: the repository-wide sweep, coverage runs, benchmarks, or anything exercising packages the diff did not touch.

State the size of anything that sweeps. A criterion implying a run over every string or every combination is bounded by saying how many there are before you run it.

Reading the pull request's checks is not a test run. The continuous integration service has already paid for that execution; you are reading a result rather than producing one.

## The branch and the tree belong to this card

The card's code goes on a branch named after the card, and you never push the trunk.

**A Dinah card has no field for a branch name.** Its fields are title, body, severity, priority and tier. So the branch name lives in the card body under the literal heading `## Branch`, on a line of its own, and every stage after you reads it from there with `dinah show <card>`. Spec records it when Spec produced a file; when the body carries no such heading, you write it before you write code, because a branch nobody recorded is a branch nobody downstream can find.

Then put the tree on the work detached, and let the remote decide where you detach:

```
git -C <tree> fetch origin
git -C <tree> ls-remote --heads origin <branch>
```

A line of output means the branch already exists, so detach onto it with `git checkout --detach origin/<branch>`. No output means it does not exist yet, so detach onto the trunk with `git checkout --detach origin/main`. That single question covers the first pass, the re-entry after a push-back, and every awkward state in between, and the recorded name never changes in any of them.

Detach rather than `git checkout -b` or `git checkout -B`. Both bind a local branch name, and each ordinary re-entry state breaks one of them. Binding a name that an earlier pass's tree still holds dies outright, and creating one from the trunk when the branch exists only on the remote succeeds while quietly tracking the trunk, after which the push is rejected with no legal way out, because you may not force-push. Detaching binds no name, so none of that can happen.

Never rebase work you have already pushed and never force-push. When the trunk has moved and your diff needs what moved, bring the trunk in with `git -C <tree> merge origin/main` and resolve there.

Commit as `git add <specific paths>`, never `git add -A`, with a message whose first line names the card and summarises the change in one line, and a body mirroring your handoff. Before handing off, bring the trunk in one last time and push:

```
git -C <tree> merge origin/main
git -C <tree> push origin HEAD:refs/heads/<branch>
```

Spell that destination out in full. The shorthand without `refs/heads/` updates a branch that already exists, but on the pass that creates one git refuses it. A bare `git push -u origin <branch>` is worse than refused: from a detached head it pushes whatever local branch carries that name, reports that everything is up to date, and exits successfully having shipped none of your work.

Never force and never skip the hooks. Confirm the branch is on the remote with `git ls-remote --heads origin <branch>`, and confirm you landed nothing on the trunk with `git log origin/main -1`.

Leave the tree clean on the way out. Run `git status --short` before you push and before you move the card; if it is not empty, either commit the change as part of this card or discard it. Test and Merge treat working-tree drift as a real gate failure.

## Every branch gets a pull request, authored by the pipeline

After the push, make sure the card's pull request exists. It is the operator's window onto the diff, and the continuous integration service runs its checks against it for free.

Create it as the pipeline identity, never as the operator. The machine account exists because a pull request's author cannot approve it, and one created under his own authentication takes the approval away. Its token lives outside every repository; never copy the token itself into any file, note, or output. Authorship follows the creator, so only the create call needs the identity, and pushes and reads stay on the default authentication. Create only when a view of the branch found nothing, because a re-entry already has its pull request and the new commits appear on it.

Either way, put the pull request's link in your handoff note so the operator and any reading tool navigate straight to it from the card. A pushed branch with no pull request is an incomplete handoff.

## Watch the checks you started

Pushing starts the checks, and this card does not leave this column until you have read what they said. A branch pushed, a pull request opened and a stage handed off inside ninety seconds is a handoff that never saw its own checks, and the next person who sees them is the operator, which is the wrong person.

Poll until every check has finished rather than reporting the first snapshot. Queued and in progress are not results.

**An empty answer is not a green answer, and it is the trap here.** A push takes time to register, and on one card no checks were reported for roughly seventeen minutes before the first appeared. Nothing-reported and nothing-failing look identical. Treat an empty list exactly as you treat a queued one and keep waiting. Wait up to thirty minutes; if checks are still absent or unfinished then, say exactly what you saw and how long you waited, file it as a question for the operator, and name the gap in the first line of your handoff so the next reader knows the card arrives unverified.

A check that fails is yours to fix now, before the handoff. Read the failing job's log and quote the assertion rather than the job name. The one exception is a failure you can show was already red on the trunk before this branch existed: name it in your cuts with the evidence so the reviewer weighs it instead of rediscovering it. A job that cannot block a merge still counts.

## Reference verification

Every user-facing string naming a destination, meaning a link, a command, a settings location, a card reference or a "see such-and-such", must point at something that exists in the codebase or in this diff. Otherwise correct the copy, or file the destination as a follow-on card and record its absence in your cuts.

### This column holds on the way out

The decisions you take in the course of the work are settled here, so the card does not leave carrying one still pending. File such a decision with `--column implement`. The refusal is `dinah.unresolved-item-exit` and it applies to a push-back exactly as it applies to an advance.

One rule places every item you file, and it runs in both directions. An item is filed against the column that settles it, and that column holds on the way out: that is why your own decisions name Implement and why a question for the operator names his station rather than yours. An item that must already be settled before a station is reached is filed against that station, and that station holds on the way in: that is why an acceptance criterion names Merge. An exit hold reads only the column the card is leaving, so an item naming a column the card has already passed holds nothing at all.

### Exits

**Forward** to Agent Code Review. The handoff note carries `## WHAT SHIPPED` and `## WHAT I CUT` as headings on their own lines, the pull request's link, the verdict of every check on it, and any pending questions stamped for the operator, so the reviewer and then the operator see them coming. The cuts are the reviewer's and the tester's map of what is not verified.

**Back** to Design Queue, with a note. **Halt** by blocking in place for a ruling the work cannot proceed without.
