---
title: Operator Review
kind: work
operator_owned: true
---
The operator's own work station, for the reviews that are expected rather than exceptional. A card arrives here because it produced something the operator has to accept before anything downstream commits to it: a UX sketch (a command transcript or --help text), an external interface or published contract, a change to the board's own flow, pricing or positioning copy, a schema customers will depend on. On another board it might be a vendor quote or a permit drawing. The operator knows these are coming, knows the card waits until they look, and can batch them.

This is not where unexpected decisions go. When an agent hits a ruling it cannot get and did not anticipate, that is the andon cord: `block_card(card_id, kind="operator_decision", reason=<the call, posed so the operator can answer it without opening the card>)`, which halts the card where it stands. Both mechanisms end with the operator, and they mean different things, so keep them apart. A block says work stopped. Arriving here says work reached a planned checkpoint.

### What admits a card, and the test that decides it

Approving something is providing a test. When the operator accepts an artifact, the acceptance criterion becomes "the implementation hews to what was approved", and that criterion is verified downstream like any other.

So the agent sending a card here must be able to state, in one line, **what acceptance criterion the operator's approval would create**. If it cannot, there was nothing to approve and the card does not belong here. Put that sentence in the move-note; it is what the operator reads first and what the verifying stage checks against later.

Judge this from what the card actually produces, not from whether the operator would find it interesting. The error is asymmetric: admitting a card that did not need it costs one glance and one click, while failing to admit one means an implementation commits to a form nobody approved and no criterion downstream will catch it. **When in doubt, admit.**

### The operator accepts the artifact, not a description of it

**An approval given without seeing the artifact is not an approval, and nobody downstream can tell the difference.** The criterion that acceptance mints says the implementation hews to what was approved. If the operator ruled on a summary, that criterion pins the implementation to something they never saw, and the artifact becomes authoritative by default rather than by decision.

So a card whose artifact is a file must say where the file is **and what it shows**, concretely enough that the operator knows what they are being asked to look at before they open it. Name the sections, name what differs between the options, and name the one detail that decides it. A path on its own is not that: it tells the operator a file exists and leaves the work of finding out what is in it to them, which is exactly the friction that produces a ruling given on prose.

If the operator rules without having looked, say so and offer to re-open. A ruling is cheap to confirm and expensive to discover was uninformed three stages later.

### Sending a card here: name where it goes next

This column sits **off every lane**, so it has no next stage of its own and cannot work one out. The agent that sends a card here must therefore name, in the same move-note, the column the card should go to once the operator accepts. That is normally the stage that would have followed the sender on this card's lane, which the sender reads from the lane block on its own claim response and the operator cannot.

A move-note that names no onward destination leaves the card stranded, so the operator's fallback is to return it to whichever column sent it and let that column route it again. That works, and it costs an extra hop, which is why naming the destination is the sender's job.

### The work

The operator claims the card, reads the artifact, and either accepts or does not. The dark-work rule applies here as everywhere: claim while reviewing, so the board says what is happening.

**On acceptance**, record the criterion as a real `acceptance_criterion` checklist item naming the artifact ("the shipped command output matches the approved transcript"), then move the card to the destination the sender named. The criterion is the point of the visit; a move without one leaves the implementer guessing at a form that has already been decided.

Check before recording it that the criterion actually discriminates. A criterion both candidate options would satisfy is not the test the approval was supposed to create, and it will pass whichever one gets built.

**On rejection**, move the card back to the column that sent it, with what is wrong stated concretely enough to act on.

Age here is decision latency, and it measures the operator rather than any supplier, because this is a queue the operator works rather than one the board waits on. Keep it short by batching.

### Skipping

A card may skip this column when speed matters more than the approval, and a card that skipped it can still pull the andon cord if something unexpected arises. A skipped review is not neutral: the implementation proceeds with no criterion governing its form, so say so in the move-note that skips it rather than passing over in silence.
