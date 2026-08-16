---
title: Merge
kind: work
operator_owned: false
---
Merge the card's branch into the trunk. This is the integration, not a confirmation of one: until you run it, the card's code is on its branch and the trunk has never seen it. Merge is a mechanical land-and-record gate rather than a review or sign-off gate. Agent Code Review, the operator stations and Test are the per-card review, and Acceptance is the operator's last look.

### The merge

Read `branch_name` from the card. It decides which of two jobs you have.

**A card with a branch gets merged through its pull request.** Work in your isolation worktree, never in the operator's checkout:

```
git fetch origin
gh pr view <branch_name> --json number,url,state
gh pr merge <branch_name> --squash --delete-branch --subject "<human-id>: <one-line summary>" --body "<mirrors WHAT SHIPPED>"
git fetch --prune origin
git log origin/main -1 --format="%H %s"
```

One squashed commit per card, so the trunk reads as one line per card and the hash you record is the card's whole footprint. `--delete-branch` removes the remote branch as part of the merge, and GitHub closes the PR with the operator's approval attached to it, which is the record the pull-request discipline exists to create. Verify the deletion with `git branch -r --list "*<human-id>*"` after the prune, which must come back empty, and confirm the squash commit is the trunk tip. Leave `branch_name` on the card: it is the record of where the work happened, and it stays true after the branch is gone.

A branch with no PR is a process miss rather than a blocker: Implement was told to open one. Merge it by the manual path below, and report the absence in the move-note so the miss is visible.

**The manual path, for a branch with no PR:**

```
git fetch origin
git checkout --detach origin/main
git merge --squash origin/<branch_name>
git commit -m "<human-id>: <one-line summary>"   # body mirrors WHAT SHIPPED
git push origin HEAD:main
git push origin --delete <branch_name>
git branch -D <branch_name>                       # best-effort housekeeping
```

On this path the remote deletion is the deletion, and the local one is allowed to fail in two expected ways: `error: branch '<name>' not found` in a fresh worktree that never checked the branch out, and `error: cannot delete branch '<name>' used by worktree at '<path>'` when the implementer's worktree still holds it, which is the normal case. Neither blocks the merge and neither is yours to resolve; do not remove somebody else's worktree to force it.

Two non-conflict outcomes of the manual push are expected too. A `git push origin HEAD:main` rejected non-fast-forward means another card merged in the meantime: re-fetch, re-detach at the new `origin/main`, re-run `git merge --squash`, re-commit and re-push. `git pull` on the detached HEAD is forbidden, whatever git's own hint text recommends, because it lands a two-parent commit on the trunk; it will not even run there, since git answers a detached HEAD with `You are not currently on a branch`. And when `git commit` reports `nothing to commit, working tree clean` straight after a clean `git merge --squash`, the branch is already contained in the trunk, which is what a re-entry looks like when an earlier attempt landed its push before it finished the deletions. Read that as a merge that already happened rather than one that failed: recover the hash with `git log --oneline --grep=<human-id> origin/main`, record it, finish the deletions, and say so in the move-note. Never reach for `--allow-empty`. The same already-merged reading applies when `gh pr view` reports the PR is MERGED before you touched it.

**A card with no branch gets confirmed.** Its code is already on `origin/main` under a trunk-pushing policy. Fetch, confirm the commits carrying the card's human-ID prefix are on `origin/main`, and record the hash. That is the whole gate for such a card.

### What to check before you merge

Spot-check the message (human-ID prefix, mirrors WHAT SHIPPED) and the diff footprint: no secrets, no accidental large files, nothing outside the card's stated scope. `git diff origin/main...origin/<branch_name>` with `--stat` is the cheap read.

**A card that arrived on the Fix lane skipped Test.** That lane runs Build Queue to Implement to Agent Code Review to Merge, so a reader has seen the diff but nothing ran it, and you are the last stop before it lands on the trunk. Read the diff against Fix-lane eligibility: one mechanical change, no schema, no migrations, no auth, no published contract, no second behavioural surface. A diff outside that boundary has outgrown its route and goes back to Build Queue to take the Build lane. You are not judging correctness, which would make this a review stage with a loop; you are judging whether the card was entitled to skip the stage that would have run it.

**A merge conflict is not yours to resolve.** `gh pr merge` refuses a conflicting PR exactly as raw git would. `block_card(card_id, kind="other", reason="merge conflict merging <branch_name> into main: <files>")` and stop. The operator resolves on the branch and unblocks. The implementer is told to keep the trunk merged into the branch, so a conflict here means something changed under them.

**Judge working-tree drift by whether it belongs to this card.** An uncommitted or untracked change that is part of this card's work is a real gate failure: the card is not landed, so push back. A stray file with nothing to do with this card is somebody else's mess, and it is not this card's problem to be held hostage by. Report it in the move-note, name the file, and carry on; never commit it as part of this card, and never revert it, because you do not know who is mid-work on it.

### Exit

**Take the destination from the lane, never from this column.** Every lane currently places Acceptance after Merge, but read the lane block on your claim response rather than trusting that sentence.

Move with:

```
COMMIT
<hash> (origin/main): <one-line summary>
[PR #<n>](<url>)
```

The hash is the squash commit, the PR link is the one the merge closed (omit the line only on a no-PR or no-branch card, and say why). Say which path ran (PR merge, manual, or confirm-only), name the branch that was deleted, and note anything expected-but-odd you saw.

A question for the operator raised at this stage blocks the card in place; both operator stations are behind you.

A missing commit gets pushed before claiming, never with `--force`. Secrets or a wrong landing: don't rewrite history from Merge; push back to Implement/Ready with the cleanup described.
