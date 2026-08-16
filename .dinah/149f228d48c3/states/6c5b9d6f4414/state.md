---
title: Merge
kind: work
operator_owned: false
---
Merge the card's branch into the trunk. This is the integration, not a confirmation of one: until you run it, the card's code is on its branch and the trunk has never seen it. Merge is a mechanical land-and-record gate rather than a review or sign-off gate. Code Review and Test are the per-card review, and Acceptance is the operator's.

### The merge

Read `branch_name` from the card. It decides which of two jobs you have.

**A card with a branch gets merged.** Work in your isolation worktree, never in the operator's checkout:

```
git fetch origin
git checkout --detach origin/main
git merge --squash origin/<branch_name>
git commit -m "<human-id>: <one-line summary>"   # body mirrors WHAT SHIPPED
git push origin HEAD:main
git log origin/main -1 --format="%H %s"
```

One squashed commit per card, so the trunk reads as one line per card and the hash you record is the card's whole footprint. Then delete the branch:

```
git push origin --delete <branch_name>    # this is the one that matters
git branch -D <branch_name>               # best-effort, see below
```

Delete only a branch you just merged and verified. Leave `branch_name` on the card: it is the record of where the work happened, and it stays true after the branch is gone.

**The remote deletion is the deletion. The local one is housekeeping and is allowed to fail**, in two ways that are both expected rather than problems. `error: branch '<name>' not found` happens in a fresh isolation worktree that never checked the branch out. `error: cannot delete branch '<name>' used by worktree at '<path>'` happens when the implementer's worktree still exists and still holds it, which is the normal case, since harness worktrees outlive the stage that made them. Neither blocks the merge and neither is yours to resolve; do not remove somebody else's worktree to force it. Report which one you saw, and verify the deletion that counts with `git fetch --prune` followed by `git branch -r --list "*<human-id>*"`, which must come back empty.

Two non-conflict outcomes of the push are expected too. A `git push origin HEAD:main` rejected non-fast-forward means another card merged in the meantime: re-fetch, re-detach at the new `origin/main`, re-run `git merge --squash`, re-commit and re-push. `git pull` on the detached HEAD is forbidden, whatever git's own hint text recommends, because it lands a two-parent commit on the trunk. It will not even run there, since git answers a detached HEAD with `You are not currently on a branch`.

A third expected outcome sits on the same path. When `git commit` reports `nothing to commit, working tree clean` straight after a clean `git merge --squash`, the branch is already contained in the trunk, which is what a re-entry looks like when an earlier attempt landed its push before it finished the deletions. Read that as a merge that already happened rather than one that failed: recover the hash with `git log --oneline --grep=<human-id> origin/main`, record it, finish the deletions, and say in the move-note that the merge was already on the trunk. Never reach for `--allow-empty`.

**A card with no branch gets confirmed.** Its code is already on `origin/main` under a trunk-pushing policy. Fetch, confirm the commits carrying the card's human-ID prefix are on `origin/main`, and record the hash. That is the whole gate for such a card.

### What to check before you merge

Spot-check the message (human-ID prefix, mirrors WHAT SHIPPED) and the diff footprint: no secrets, no accidental large files, nothing outside the card's stated scope. `git diff origin/main...origin/<branch_name> --stat` is the cheap read.

**A card that arrived on the Fix lane skipped Test.** That lane runs Build Queue to Implement to Code Review to Merge, so a reader has seen the diff but nothing ran it, and you are the last stop before it lands on the trunk. Read the diff against Fix-lane eligibility: one mechanical change, no schema, no migrations, no auth, no published contract, no second behavioural surface. A diff outside that boundary has outgrown its route and goes back to Build Queue to take the Build lane. You are not judging correctness, which would make this a review stage with a loop; you are judging whether the card was entitled to skip the stage that would have run it.

**A merge conflict is not yours to resolve.** `block_card(card_id, kind="other", reason="merge conflict merging <branch_name> into main: <files>")` and stop. The operator resolves on the branch and unblocks. The implementer is told to keep the trunk merged into the branch, so a conflict here means something changed under them.

**Judge working-tree drift by whether it belongs to this card.** An uncommitted or untracked change that is part of this card's work is a real gate failure: the card is not landed, so push back. A stray file with nothing to do with this card is somebody else's mess, and it is not this card's problem to be held hostage by. Report it in the move-note, name the file, and carry on; never commit it as part of this card, and never revert it, because you do not know who is mid-work on it.

### Exit

**Take the destination from the lane, never from this column.** Every lane currently places Acceptance after Merge, but read the lane block on your claim response rather than trusting that sentence.

Move with:

```
COMMIT
<hash> (origin/main): <one-line summary>
```

The hash is the squash commit you pushed, or, for a card with no branch, the commit that was already there. Say which of the two happened, name the branch you deleted, and say which of the local-delete outcomes you saw.

A missing commit gets pushed before claiming, never with `--force`. Secrets or a wrong landing: don't rewrite history from Merge; push back to Implement/Ready with the cleanup described.
