---
title: Operator Code Review
slug: operator-code-review
kind: work
reject_to: implement
operator_owned: false
gate_items: out
---
The operator's code-side station, the twin of Operator Design Review on the build half. A card arrives here after Agent Code Review, and what happens next depends on what it brought.

The diff review happens on the card's pull request. The arriving move note carries its link, so he navigates straight to the diff, reads it there, and records approval or requested changes on the pull request itself. The workbench mirrors the verdict: approval moves the card forward to Test, requested changes move it back to Implement, which this column declares as its reject target, with the requests summarised in the note. A code card arriving without a link is an incomplete handoff, and Agent Code Review was told to catch it.

A card carrying pending open questions stamped for him waits for him to answer them. Implement and Agent Code Review file their questions against this column rather than blocking, whenever the work can be finished despite the question. A card carrying an artifact he must accept before it ships, meaning changed command output, a published surface, or copy somebody outside the project will read, waits for that acceptance.

A card carrying neither does not stop here at all. He ruled on 2026-09-14 that this station lets a clean card through: ownership is off, and the hold is on the way out, so the card is refused only while it carries a pending item naming this column.

### How the stop is made, now that the station is not owned

The hold reads the card's items and never the actor, so it refuses him by name, as `dinah.unresolved-item-exit`, exactly as it refuses anybody else. That is the whole of what stops a card here. A question nobody filed against this column is a question this station cannot hold a card for, which is why a sender's `--column operator-code-review` matters as much as the owner stamp.

He can lift his own stop deliberately, because an override is restricted to the operator and the move records that it was overridden.

### What he gave up in choosing this shape

He does not see a clean card's diff. Between Implement and the trunk there is then nothing but an agent choosing to file an item. He was told that plainly and chose it, and it is not reopened here. What remains true is that Agent Code Review reads every diff on every card, and that the hold still refuses his own move out while his own question on that card is unsettled, which is the failure the hold was built to stop.

### The work

He claims the card with `dinah claim <card>` before reading it, because a move leaves a card ready rather than held.

Answer each pending question stamped for him with `dinah resolve <item> "<the ruling and its reasoning>"`. Downstream stages act on the note, so it carries the ruling rather than a bare yes or no. An item stamped for the operator can be closed only by him.

On acceptance of an artifact, record the criterion with `dinah file <card> acceptance_criterion "<what was approved>" --column merge`, so it is held at the entry to Merge like every other criterion. One rule decides that column, and it runs in both directions. An item is filed against the column that settles it, and that column holds on the way out, which is why a question for him names this station. An item that must already be settled before a station is reached is filed against that station, and that station holds on the way in, which is why a criterion names Merge and not this column. An exit hold reads only the column the card is leaving, so an item naming a column the card has already passed holds nothing at all. On rejection, move the card back to Implement with what is wrong stated concretely enough to act on. Otherwise move it forward to Test.

### What the senders owe this station

Implement and Agent Code Review say in their move notes what awaits here: the pull request's link, the questions listed, the one-line acceptance criterion an approval would create, or the plain statement that nothing does. A card arriving with no such note costs him the read the sender should have done, and on a clean card it costs him the card entirely, because a clean card does not stop.

Age here is decision latency and measures the operator; keep it short by batching.
