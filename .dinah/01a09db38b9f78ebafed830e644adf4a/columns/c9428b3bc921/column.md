---
title: Test
slug: test
kind: work
reject_to: implement
operator_owned: false
gate_items: out
---
Validate the implementation against the card's contract and its acceptance criteria by running it. This is the workbench's one authoritative execution stage and the last gate before Merge puts the code on the trunk, so the repository-wide sweep runs here and only here, when it is warranted.

### Put the tree on the branch, then bring the trunk into it

The card's code is on the branch named in its body under the `## Branch` heading, and it is not in the trunk, so a tree left on the trunk verifies nothing at all. Before you run anything:

```
git -C <tree> fetch origin
git -C <tree> checkout --detach origin/<branch>
git -C <tree> merge origin/main
```

Detach rather than binding the branch name, because git refuses to check out a branch another tree already holds and the implementer's tree usually still holds this one. Detaching gets you the same tree with no collision.

The merge is not housekeeping. Nobody tests the trunk with this card in it before Merge lands it, so the merged state is what you are being asked about, and testing the branch alone would pass a card that breaks on contact with whatever shipped while it sat in review. Two branches can each pass their own suite and fail together, because a rule landing on one and the data violating it landing on the other meet for the first time on the trunk, and that has happened here four times. A merge can also break a build with no conflict marker at all, when one branch's fixture predates a member another branch adds, so build the merged result rather than reading the merge.

If the merge conflicts, push the card back to Implement, which this column declares as its reject target, with the conflicting files named. If it changed anything, push it so Merge and every later reader see the same tree you tested. If git reports that it was already up to date, nothing changed and there is nothing to push; say so rather than pushing an empty update.

### Choose the scope deliberately, then name what you ran

Targeted is the default. Identify the packages the diff touched, then run `go test` on those and their direct dependents. Reserve the full sweep for cards touching storage format or migrations, shared infrastructure, cross-package renames, or the command surface. Your report says which one you ran and why.

One package reads the whole tree while being a package almost no diff names, so a self-test scoped to the packages the diff touched will not run it. Run it regardless of what the diff names.

After any merge resolution, recompute rather than reconcile, and run the whole suite rather than the packages the diff touched. One fixture in this repository still keys on source line numbers and shifts when a diff adds or removes lines above it, with nothing announcing it. Open each anchored line and read it rather than checking the arithmetic.

### The card's new tests have to run inside the sweep you executed

Confirm the new tests actually ran rather than assuming they did. Where an existing test already covered the area, the new coverage belongs inside it; a parallel file testing the same ground twice is duplication every future card pays for, and you report it rather than letting it settle.

### Criteria already marked verified are the thing to be careful about

By the time a card reaches you the implementer has marked its criteria and a reviewer has accepted those marks by reading. That is exactly the condition where an execution stage earns its keep by being suspicious instead of confirmatory. Re-verify by running, and re-mark anything whose evidence does not survive an actual run with `dinah fail <item> "<expected versus actual>"`. A criterion marked verified on a thin note is a finding.

Watch for the shapes that pass without proving anything. An assertion matching a substring the old behaviour also produced tells you nothing about the change. An expected value computed by calling the thing under test moves with the defect and can never fail. A sweep over an empty set reports success, so assert how big the set was, and give each half of a two-part sweep its own count. A run piped into another command reports the pipe's exit status rather than its own. Any of these is a failed criterion rather than a passing one.

Where a check sweeps a set, say how big the set is before you run it. One card here cost hours by sweeping without a bound, and another ran thirty-seven thousand strings at eighteen minutes a pass, which was worth it only because the number was known in advance.

### Verification kinds, as the card demands

Package tests; migration runs on a fresh store and on a store in the pre-migration shape where the card touches the storage format; and direct runs of the affected commands where the change touches command output, flags or help text. Pick up any directed targets the Agent Code Review findings comment named, because those are contract.

User-facing output gets realistic shapes rather than the happy path alone: empty, a single row, the boundary at any cap, overflow past the cap so any "and more" affordance fires, and very long text. A criterion mentioning a cap or a top-of-list is only verified once the overflow condition actually ran.

### Every question you file names its owner, and one kind does not travel

You are past both operator stations here. **A question for the operator therefore does not travel with the card: it blocks the card in place**, with `dinah block <card> "<the question>" --kind operator-ruling`, and he answers and unblocks it there. Filing it against a column behind the card would hold nothing, because an exit hold reads only the column the card is leaving.

Before you file a question that does ride the card, place it in one of two classes and stamp it. **A later stage's**, meaning Merge or Acceptance decides it as a matter of course: file it with `--owner holder` and the column that settles it, and say what that stage needs in order to decide. A criterion that genuinely cannot close until the change is on the trunk belongs here rather than being marked failed: leave it pending and say it closes at Acceptance. **Another card's**: settle it here with a note stating the follow-up at the quality of a title, so it can be filed without re-reading your report.

Report the counts: how many you filed, and whether anything blocked.

### This column holds on the way out

A decision a tester takes in the course of the work is settled here, so file it with `--column test` and settle it before the card leaves. Acceptance criteria are not held here: they name Merge and are held at its entry, because this is the station that verifies them and a criterion held here would have to be verified before the column that verifies it.

### Report and route

Mark each acceptance criterion with `dinah verify <item> "<what was checked>"` or `dinah fail <item> "<expected versus actual>"`. Post a report comment carrying the status, the scope you chose and why, a sample of the output, the failures, and the criteria you verified and left pending. A pass moves the card to Merge. A failure pushes it back to Implement. A problem outside this card's scope becomes a separate card, unless it is something you can clear here, in which case clear it and say so.

### What this stage cannot cover

Every card is tested on its own branch with the trunk merged in as of the moment you merged it. A failure appearing only once this card sits alongside siblings that landed after that merge is invisible here by construction. Acceptance exercises the integrated trunk for exactly that reason.
