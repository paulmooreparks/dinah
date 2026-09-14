---
title: Acceptance
slug: acceptance
kind: work
reject_to: implement
operator_owned: true
---
Cards whose commit is on the trunk: delivered, but not yet proven in a running build.

A card sits here as the operator's awareness that the change is out and living in whatever build carries it. Passing Acceptance is his acknowledgement that nothing untoward happened as a result of the card shipping. It is an acknowledgement rather than an investigation: nobody assembles an evidence file here, and a quiet build is itself the evidence.

### This column is his outright

Acceptance is the one column on this workbench that is owned by the operator, and it holds neither way. Ownership refuses a move out to everybody who is not him, so every card stops here unconditionally, which is what he asked for at the station where his judgement is the last word. A hold on top would additionally refuse his own move out while an item naming Acceptance sat pending, and that is a second stop he did not ask for.

Ownership refuses the departure rather than the arrival, so a card arrives here freely and then waits for him.

He claims the card while judging it, with `dinah claim <card>`. The rule against working an unclaimed card covers him too.

### A question raised here does not travel

Both operator review stations are behind this one, so a question raised at Acceptance blocks the card in place with `dinah block <card> "<the question>" --kind operator-ruling`. Filing it against a column behind the card would hold nothing, because an exit hold reads only the column the card is leaving.

A card arriving here still carrying a pending question stamped for the operator was misrouted. Questions are answered at his review stations, or at the block where they arose, and never presented for acceptance. If you are the operator, answer it before judging the card; anybody else blocks the card with the question as the reason rather than working around it.

### When disruption appears

Evidence of disruption in the corresponding build while the card sits here sends it back: to Implement, which this column declares as its reject target, when the contract was right and the change was wrong, or further back through Design Queue to Spec when the contract itself was wrong. Say what was observed in the move note.

The card that shipped the change owns the consequence, and pushing that card back rather than filing a fresh one is what keeps the link between a change and its effect, which is the reason this stage exists. A problem that turns out to predate the merge is a separate card, as it would be anywhere else.

Any acceptance criterion the card still carries, including one minted at an operator station earlier in its life, is closed honestly here or left pending with the reason recorded.

### Age here

Age in this column is decision latency and it measures the operator rather than any supplier. Keep it short by batching: acknowledge several cards in one sitting rather than one at a time.

### Leaving

Move the card to Done with `dinah move <card> done` when it has sat in a running build without incident long enough to be acknowledged. On this workbench that is the last gate, so Done means merged and accepted.
