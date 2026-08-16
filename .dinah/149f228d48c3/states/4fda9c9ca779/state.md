---
title: Implement
kind: work
operator_owned: false
---
Where specs become code in the Dinah codebase, a single-binary Go CLI. card-implement works in an isolated worktree on a branch named after the card, self-tests, pushes the branch, then moves the card on. The trunk does not see this work until Merge.

### What arrives, and what the clarity gate asks

Read the spec, the checklist and the latest move-comment before writing code. What the gate demands depends on how the card got here.

**A card that came through Spec arrives with a reviewed contract.** If that contract is ambiguous, missing or self-contradictory, do not start: push back one column with a note naming the ambiguity. If the ambiguity is a ruling only the operator can give, block in place with `kind="operator_decision"` instead.

**A card routed straight from Triage arrives with no spec, by design.** That is the fast path, and its contract is meant to be obvious from the card alone. So the gate here is different: ask whether you can state, in one sentence, exactly what is changing and how, with no judgement calls left. If you can, build it. If you cannot, the routing call was wrong rather than the card being unclear, so move it to Design Queue with a note naming what a spec would have to settle. Re-routing a misclassified card to the head of its proper path is the one sanctioned exception to the one-column-back rule; never push a spec-less card back where it came from, because nothing there will change.

Don't reinterpret silently and don't expand scope: a dependency the spec missed becomes a new card (spawned_from, workstreams inherited), and a problem bigger than this card's scope is filed separately, not fixed in the same diff.

## Delivering an implementation means delivering a test

This is not negotiable and it is not somebody else's stage. A change that ships without a test covering the behaviour it changed is an incomplete implementation. Writing the test is part of the work you were asked to do, not an addition to it.

If a test already covers the behaviour you changed, extending it satisfies this and is the preferred answer.

**Integrate, do not accumulate.** When an existing test covers the area you touched, extend that test. Create a new test file only when the behaviour genuinely has no home in the suite. A regression suite that grows by accretion becomes slow, then redundant, then something people skip, and a suite people skip protects nothing. Name in WHAT SHIPPED which existing test you extended, or say why a new one was necessary.

**Arm the test.** A test that passes proves nothing on its own, because it also passes when the thing it guards is absent. Break what you fixed, watch the test go red, restore from a byte-identical backup, and confirm green again. Report that you did it. This is how a test earns the right to be trusted later by somebody who was not here.

## Prose ships under the prose standard

Any prose this card produces (README and docs changes, user-facing copy, error text with sentences in it, and the WHAT SHIPPED note) is written to the workbench document **"Prose standard"**. Code Review holds it against that standard's tell list, so write to it the first time. When the card rewrites existing prose, the standard's hard constraint applies before style does: meaning cannot change, and the negative clause you are tempted to cut is often the requirement.

## A diff that outgrows its lane goes back rather than forward

A Fix-lane card is entitled to skip Test because its blast radius is bounded. When the work turns out to touch schema, migrations, auth, a published contract, or more than one behavioural surface, that entitlement is gone: move the card to Design Queue with a note naming what a spec would have to settle, rather than finishing it on the short path.

**Every lane on this board has a review stage.** If the lane block on your claim response shows nothing after this column but the merge, the lane is misconfigured rather than short. Say so in a note and stop; do not take up a fresh reader's obligations on your own diff, because the person who wrote the code cannot be that reader.

## The testing boundary, and what belongs to Test

Implement runs unit-level verification only. Build, run the tests covering what this card changed, and nothing more: no repo-wide sweeps, no coverage runs, no benchmarks. The full regression matrix belongs to Test, and this column can cycle with a reviewer more than once on a single card, so every second spent here is paid again on every loop.

Two exceptions apply, and only when they apply to this card:

- The arming proof above, which requires building the broken state deliberately.
- The whole sweep becomes yours when **no Test stage follows on this card's lane**, because nobody downstream will run it. Read that off the lane block on your claim response rather than assuming it from the card's shape.

The gate before handoff:

- `go build ./...` and `go vet ./...` clean. Non-negotiable.
- `go test` on the package or packages the diff touched.
- Schema changes: idempotent migration verified on a fresh database and on a pre-migration-shape database, with data preserved.
- Command-surface changes (output, flags, help text): run the affected command locally, with a note naming what was exercised.

## The branch is where this work lives

The card's code goes on a branch named after the card, and you never push the trunk. `<branch>` comes from the workbench's `Branch-pattern` directive expanded for this card, which on a board that declares nothing is the card's human ID. Record it immediately with `update_card(card_id, branch_name="<branch>")`, before you write code. That field is what every stage after you reads to find your work, and a branch nobody recorded is a branch nobody downstream can find.

Then put the worktree on the work detached, and let the remote decide where you detach:

```
git fetch origin
git ls-remote --heads origin <branch>
```

A line of output means the branch already exists, so detach onto it with `git checkout --detach origin/<branch>`. No output means it does not exist yet, so detach onto the trunk with `git checkout --detach origin/main`. That single question covers the first pass, the re-entry after a push-back, and every awkward state in between, and the recorded name never changes in any of them.

**Detach rather than `git checkout -b` or `git checkout -B`.** Both bind a local branch name, and each ordinary re-entry state breaks one of them. `git checkout -B <branch> origin/<branch>` dies with `fatal: '<branch>' is already used by worktree at '<path>'` whenever an earlier pass's isolation worktree still holds the name, and since harness worktrees outlive the stage that made them, that is the normal case rather than the rare one. `git checkout -b <branch> origin/main` dies with `fatal: a branch named '<branch>' already exists` when a previous pass created the name and was interrupted before it pushed. Worst of the three, `git checkout -b <branch> origin/main` SUCCEEDS when the branch exists only on the remote, quietly tracking the trunk instead, and the push at the end is then rejected non-fast-forward with no legal way out, because you may not force-push. Detaching binds no name, so none of that can happen.

Never rebase work you have already pushed and never force-push. When the trunk has moved and your diff needs what moved, bring the trunk in with `git merge origin/main` and resolve there.

Commit as `git add <specific paths>` (never `git add -A`), with the message `<human-id>: <one-line summary>` and a body mirroring WHAT SHIPPED. Before handing off, bring the trunk in one last time and push:

```
git merge origin/main
git push origin HEAD:refs/heads/<branch>
```

Spell that destination out in full. The shorthand `git push origin HEAD:<branch>` updates a branch that already exists, but on the pass that creates one git refuses it with `error: The destination you provided is not a full refname`. A bare `git push -u origin <branch>` is worse than refused: from a detached HEAD it pushes whatever local branch happens to carry that name, reports `Everything up-to-date`, and exits 0 having shipped none of your work.

Never `--force`, never `--no-verify`. Confirm the branch is on the remote with `git ls-remote --heads origin <branch>`, and confirm you did not land anything on the trunk with `git log origin/main -1`.

A workbench whose `Push-policy` directive says `worktree-push` keeps the older contract: commit, pull, `git push origin HEAD:main`, and record no branch name. Read the directive before you assume which one you are on.

## Reference verification

Every user-facing string that names a destination (a URL, a page, a button, a route, a command, a settings location, a card ID, a "see X") must point at something that exists in the codebase or in this diff. Otherwise correct the copy, or file the destination as a follow-on card and record its absence in WHAT I CUT.

## Exits

**Forward: take the destination from the lane, never from this column.** Read the lane block on your claim response and move to the stage after Implement on the lane this card is actually on. That is Code Review on every lane today. When lane data and prose disagree, the lane data wins.

The handoff move-note carries `## WHAT SHIPPED` and `## WHAT I CUT` as Markdown headings on their own lines (the exit gate rejects bare prefixes); the cuts are the reviewer's and tester's map of what is NOT verified.

**Back:** one column, with a note. **Halt:** block in place for an operator ruling.
