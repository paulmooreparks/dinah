---
title: Operator Design Review
slug: operator-design-review
kind: work
reject_to: agent-design-review
operator_owned: false
gate_items: out
---
The operator's design-side station. A card arrives here after Agent Design Review, and what happens next depends on what it brought.

A card carrying pending open questions stamped for the operator, or left unstamped, which reads the same, waits for him to answer them. Spec and Agent Design Review file their questions for this station rather than blocking, whenever the specification can be finished despite the question. A card carrying an artifact he has to accept before anything downstream commits to it waits for that acceptance: a command transcript, a draft of help text, an external interface, a change to how this workbench itself works, or copy somebody outside the project will read.

A card carrying neither does not stop here at all, and that is deliberate. The operator ruled on 2026-09-14 that this station lets a clean card through. Ownership is off, so any actor may move a card out, and the hold is on the way out, so the card is refused only while it carries a pending item naming this column.

### How the stop is made, now that the station is not owned

The hold reads the card's items and never the actor. It therefore refuses the operator himself, by name, as `dinah.unresolved-item-exit`, exactly as it refuses anybody else. That is the whole of what stops a card here, and it is why the sender's stamp matters: a question nobody filed against this column is a question this station cannot hold a card for.

He can lift his own stop deliberately, because an override is restricted to the operator and the move records that it was overridden. That is a recorded act rather than an accident.

### What he gave up in choosing this shape

He does not see a clean card's diff at his own stations. Between Implement and the trunk there is then nothing but an agent choosing to file an item. He was told that in those terms and chose it, and it is not reopened here. Agent Design Review still reads every specification and Agent Code Review still reads every diff.

### This is not where unexpected decisions go

When an agent hits a ruling it cannot get and did not anticipate, that is the cord to pull: `dinah block <card> "<the call, posed so it can be answered without opening the card>" --kind operator-ruling`, which halts the card where it stands. Both mechanisms end with the operator and they mean different things, so keep them apart. A block says work stopped. Arriving here says work reached a planned station.

### What the senders owe this station

Approving something is providing a test. When the operator accepts an artifact, the acceptance criterion becomes that the implementation hews to what was approved, and that criterion is verified downstream like any other.

So the stage sending a card here says in its move note what awaits: the pending questions listed, the one-line acceptance criterion an artifact's approval would create, or the plain statement that the card is clean and will pass straight through. That note is what he reads first and what the verifying stage checks against later.

### He accepts the artifact, not a description of it

An approval given without seeing the artifact is not an approval, and nobody downstream can tell the difference. The criterion that acceptance creates says the implementation hews to what was approved. If he ruled on a summary, that criterion pins the implementation to something he never saw, and the artifact becomes authoritative by default rather than by decision.

So a card whose artifact is a file says where the file is and what it shows, concretely enough that he knows what he is being asked to look at before he opens it. Name the sections, name what differs between the options, and name the one detail that decides it. A path on its own is not that.

If he rules without having looked, say so and offer to reopen. A ruling is cheap to confirm and expensive to discover was uninformed three stages later.

### The work

He claims the card with `dinah claim <card>`, reads what it brought, and acts. A move leaves a card ready rather than held, so the claim is a separate act and it is what stops somebody else working the card underneath him.

**Questions.** Answer each pending item stamped for him during the same visit, with `dinah resolve <item> "<the ruling and its reasoning>"`. Downstream stages act on the note, so it carries the ruling itself rather than a bare yes or no. An item stamped for the operator can be closed only by him, so nobody else can clear this for him. The card cannot leave while one is pending, by the hold.

**On acceptance** of an artifact, record the criterion as a real acceptance criterion with `dinah file <card> acceptance_criterion "<what was approved>" --column merge`, then move the card on. One rule decides that column, and it runs in both directions. An item is filed against the column that settles it, and that column holds on the way out, which is why a question for him names this station. An item that must already be settled before a station is reached is filed against that station, and that station holds on the way in, which is why a criterion names Merge and not this column. An exit hold reads only the column the card is leaving, so an item naming a column the card has already passed holds nothing at all. The criterion is the point of the visit; a move without one leaves the implementer guessing at a form that has already been decided. Check before recording it that the criterion actually discriminates: a criterion both candidate options would satisfy is not the test the approval was supposed to create.

**On rejection**, move the card back to Agent Design Review, which this column declares as its reject target, with what is wrong stated concretely enough to act on.

**Forward** is Build Queue.

Age here is decision latency, and it measures the operator rather than any supplier, because this is a queue he works rather than one the workbench waits on. Keep it short by batching.
