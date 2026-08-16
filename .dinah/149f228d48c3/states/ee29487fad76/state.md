---
title: Operator Code Review
kind: work
operator_owned: true
---
The operator's code-side work station, the twin of Operator Design Review on the build half of the board. Every card on a lane that carries this stage stops here after Agent Code Review, and what happens next depends on what the card brought.

The diff review happens on the card's pull request. The arriving move-note carries a `[PR #<n>](<url>)` Markdown link, so the operator navigates straight to the diff in GitHub's interface or a VS Code pull-request tool, reads it there, and records approval or change-requests on the PR itself. The board mirrors the verdict: approval moves the card forward, change-requests move it back to Implement/Ready with the requests summarised in the note. A code card arriving without a PR link is an incomplete handoff; Agent Code Review was told to catch it, and the fallback is `gh pr view <branch_name>` against the card's recorded branch.

A card carrying pending open questions stamped `owner="operator"` waits for the operator to answer them; Implement and Agent Code Review file their questions for this station rather than blocking, whenever the work can be finished despite the question. A card carrying an artifact the operator must accept before it ships (changed command output, a published surface, customer-facing copy) waits for that acceptance. A card carrying neither is the common case, and the operator reads the PR, approves, and moves it on.

### The work

Claim while reviewing; the dark-work rule covers operators too. Answer each pending `owner="operator"` question (an unstamped one reads the same) with `update_checklist_item(item_id, state="resolved", note=<the ruling and its reasoning>)`; downstream stages act on the note, so it carries the ruling itself rather than a bare yes or no. A card may not leave this column with such a question still pending.

On acceptance of an artifact, record the criterion as a real `acceptance_criterion` checklist item naming what was approved. On rejection, move the card back to Implement/Ready with what is wrong stated concretely enough to act on. Otherwise move the card forward to the lane's next stage, which the lane block names: Test on the Build lane, Merge on the Design lane.

### What the senders owe this station

Implement and Agent Code Review say in their move-notes what awaits here: the `[PR #<n>](<url>)` link, the questions listed, the one-line acceptance criterion an approval would create, or the plain statement that nothing does. A card arriving with no such note costs the operator the read the sender should have done.

### Lanes and traversals that skip this station

The Fix lane never stops here, and a fast-tracked traversal is entitled to skip this column by directive. On those routes a question for the operator does not travel: `block_card(kind="operator_decision")` where the question arose, and the operator answers and unblocks it there. The PR still exists on every code card, so the operator can read any Fix-lane diff from the PR list even though the card never stopped. Age here is decision latency and measures the operator; keep it short by batching.
