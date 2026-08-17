---
title: Operator Design Review
slug: operator-design-review
kind: work
operator_owned: true
---
The operator's design-side work station, an ordinary stage on every lane that runs the design half. A card arrives here after Agent Design Review, and what happens next depends on what it brought.

A card carrying pending open questions stamped `owner="operator"` (or left unstamped, which reads the same) waits for the operator to answer them; Spec and Agent Design Review file their questions for this station rather than blocking, whenever the spec can be finished despite the question. A card carrying an artifact the operator has to accept before anything downstream commits to it waits for that acceptance: a UX sketch (a command transcript or --help text), an external interface or published contract, a change to the board's own flow, pricing or positioning copy, a schema customers will depend on; on another board it might be a vendor quote or a permit drawing. A card carrying neither is the common case, and the operator simply moves it on with one glance and one click, minting no criterion.

This is not where unexpected decisions go. When an agent hits a ruling it cannot get and did not anticipate, that is the andon cord: `block_card(card_id, kind="operator_decision", reason=<the call, posed so the operator can answer it without opening the card>)`, which halts the card where it stands. Both mechanisms end with the operator, and they mean different things, so keep them apart. A block says work stopped. Arriving here says work reached a planned station.

### What the senders owe this station

Approving something is providing a test. When the operator accepts an artifact, the acceptance criterion becomes "the implementation hews to what was approved", and that criterion is verified downstream like any other.

So the stage sending a card here says in its move-note what awaits: the pending questions listed, the one-line acceptance criterion an artifact's approval would create ("the shipped command output matches the approved transcript on the card's branch"), or the plain statement that the card is clean and one click sends it on. That note is what the operator reads first and what the verifying stage checks against later; a card arriving without it costs the operator the read the sender should have done.

### The operator accepts the artifact, not a description of it

**An approval given without seeing the artifact is not an approval, and nobody downstream can tell the difference.** The criterion that acceptance mints says the implementation hews to what was approved. If the operator ruled on a summary, that criterion pins the implementation to something they never saw, and the artifact becomes authoritative by default rather than by decision.

So a card whose artifact is a file must say where the file is **and what it shows**, concretely enough that the operator knows what they are being asked to look at before they open it. Name the sections, name what differs between the options, and name the one detail that decides it. A path on its own is not that: it tells the operator a file exists and leaves the work of finding out what is in it to them, which is exactly the friction that produces a ruling given on prose.

If the operator rules without having looked, say so and offer to re-open. A ruling is cheap to confirm and expensive to discover was uninformed three stages later.

### The work

The operator claims the card, reads what it brought, and acts. The dark-work rule applies here as everywhere: claim while reviewing, so the board says what is happening.

**Questions:** answer each pending `owner="operator"` item during the same visit, with `update_checklist_item(item_id, state="resolved", note=<the ruling and its reasoning>)`. Downstream stages act on the note, so it carries the ruling itself rather than a bare yes or no. A card may not leave this column with such a question still pending.

**On acceptance** of an artifact, record the criterion as a real `acceptance_criterion` checklist item naming the artifact, then move the card forward. The criterion is the point of the visit; a move without one leaves the implementer guessing at a form that has already been decided. Check before recording it that the criterion actually discriminates: a criterion both candidate options would satisfy is not the test the approval was supposed to create, and it will pass whichever one gets built.

**On rejection**, move the card back to the column that sent it, with what is wrong stated concretely enough to act on.

**Forward** is the lane's next stage, Build Queue on both lanes that run this station.

Age here is decision latency, and it measures the operator rather than any supplier, because this is a queue the operator works rather than one the board waits on. Keep it short by batching.

### Routes that skip this station

The Fix lane never stops here, and a fast-tracked traversal is entitled to skip this column by directive. On those routes a question for the operator does not travel: `block_card(kind="operator_decision")` where the question arose, and the operator answers and unblocks it there. Questions raised downstream have their own station: Implement and Agent Code Review file for Operator Code Review, and Test, Merge and Acceptance always block in place. Nothing arrives at Acceptance carrying a pending operator-owned question; a card that does was misrouted, and the fix is routing, never acceptance-around-the-question.
