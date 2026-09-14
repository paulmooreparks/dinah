---
title: Merge
slug: merge
kind: work
reject_to: implement
operator_owned: false
gate_items: true
---
Merge the card's branch into the trunk. This is the integration, not a confirmation of one: until you run it, the card's code is on its branch and the trunk has never seen it. Merge is a mechanical land-and-record gate rather than a review gate. Agent Code Review, the operator's station and Test are the per-card review, and Acceptance is his last look.

### This column holds on the way in

An acceptance criterion names Merge and the card is refused entry while the criterion is unverified. The refusal is `unresolved-item`. That is the operator's own ruling of 2026-09-01, and the reason it sits here rather than one column earlier is that Test is the station that verifies criteria, so holding at Test would require a criterion to be verified before the column that verifies it. The card still cannot leave Test with a criterion unverified, which is what the rule was for.

So a card arriving refused is a card whose criteria Test did not close. Read them with `dinah show <card>`, and send it back to Implement, which this column declares as its reject target, or back to Test, rather than overriding the hold.

### The merge

Read the branch name from the card body, under the `## Branch` heading. It decides which of two jobs you have.

**A card with a branch gets merged through its pull request.** Work in your own tree, never in the operator's checkout. Fetch, read the pull request, squash-merge it with a subject naming the card and a body mirroring the implementer's handoff, delete the remote branch as part of the merge, prune, and read the resulting commit back off the trunk.

One squashed commit per card, so the trunk reads as one line per card and the hash you record is the card's whole footprint. Deleting the branch as part of the merge is what closes the pull request with the operator's approval attached to it, which is the record the pull-request discipline exists to create. Verify the deletion after the prune, and confirm the squash commit is the trunk tip. Leave the `## Branch` heading on the card: it is the record of where the work happened, and it stays true after the branch is gone.

**The trunk accepts no direct push.** The repository requires a pull request with its checks green, blocks force pushes, and restricts deletion of the default branch, so the pull request is the only door. A branch with no pull request is therefore a process miss rather than a blocker: create the missing one yourself under the pipeline identity with the title and body it should have carried, wait for the checks to finish, merge it by the path above, and report the miss in your move note so it stays visible.

Two outcomes are expected rather than problems. A pull request already merged before you touched it is a re-entry after an earlier attempt landed the merge but not the bookkeeping: recover the hash from the trunk's log, record it, and say so. And a refusal because a required check is still running is a wait rather than a failure, so poll and merge when they settle.

**A card with no branch gets confirmed.** Its deliverable was not code, or its code is already on the trunk. Confirm what the card claims landed and record it. That is the whole gate for such a card.

### What to check before you merge

Spot-check the commit message and the diff footprint: no secrets, no accidental large files, nothing outside what the card said it was doing. A read of the diff with a summary of the changed files is the cheap version.

**Check that the branch actually carries the trunk.** A pull request reported as mergeable with green checks proves nothing about whether the branch predates what has since landed. Confirm the trunk is an ancestor of the branch before you merge, because green on a stale branch is green on a question nobody asked.

**A merge conflict is not yours to resolve.** Block the card with the conflicting files named and stop. The operator resolves on the branch and unblocks. The implementer is told to keep the trunk merged into the branch, so a conflict here means something changed under him.

**Judge working-tree drift by whether it belongs to this card.** An uncommitted change that is part of this card's work is a real gate failure, because the card is not landed, so push it back. A stray file with nothing to do with this card is somebody else's, and it is not this card's problem to be held hostage by. Report it, name the file, and carry on; never commit it as part of this card, and never revert it, because you do not know who is mid-work on it.

### A question raised here does not travel

Both operator stations are behind you, so a question for him blocks the card in place with `dinah block <card> "<the question>" --kind operator-ruling`. Filing it against a column behind the card would hold nothing.

Re-running a release is his call and never a stage's, because it mints a tag and publishes.

### Exit

Forward to Acceptance, with a move note carrying the trunk commit and the pull request's link, saying which path ran, naming the branch that was deleted, and noting anything expected but odd that you saw.

A missing commit gets pushed to the card's branch before you claim it, never with force. Secrets or a wrong landing do not get history rewritten from here: push the card back to Implement with the cleanup described.
