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

**The trunk accepts no direct push.** The repository's ruleset requires a pull request with the four status checks green, blocks force pushes, and restricts deletion of the default branch, with an empty bypass list, so the pull request is the only door and the remote refuses everything else. A branch with no PR is therefore a process miss rather than a blocker: Implement was told to open one. Create the missing PR yourself under the pipeline identity, `GH_TOKEN="$(cat "$HOME/.dinah-gh-token")" gh pr create --head <branch_name> ...` with the title and body a PR should have carried, wait for the required checks to finish, merge it by the PR path above, and report the miss in the move-note so it stays visible.

Two outcomes of the PR path are expected rather than problems. `gh pr view` reporting the PR already MERGED before you touched it is a re-entry after an earlier attempt landed the merge but not the bookkeeping: recover the hash with `git log --oneline --grep=<human-id> origin/main`, record it, and say so in the move-note. And `gh pr merge` refusing because a required check is still running is a wait, not a failure: the checks take about a minute, so poll `gh pr checks <branch_name>` and merge when they settle.

**A card with no branch gets confirmed.** Its code is already on `origin/main` under an older trunk-pushing policy. Fetch, confirm the commits carrying the card's human-ID prefix are on `origin/main`, and record the hash. That is the whole gate for such a card.

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

The hash is the squash commit, the PR link is the one the merge closed (omit the line only on a no-branch card, and say why). Say which path ran (PR merge, created-then-merged, or confirm-only), name the branch that was deleted, and note anything expected-but-odd you saw.

A question for the operator raised at this stage blocks the card in place; both operator stations are behind you.

A missing commit gets pushed to the card's BRANCH before claiming, never with `--force`. Secrets or a wrong landing: don't rewrite history from Merge; push back to Implement/Ready with the cleanup described.
