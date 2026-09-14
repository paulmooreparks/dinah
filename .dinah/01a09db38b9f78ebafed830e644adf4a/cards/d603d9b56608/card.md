---
title: A checklist item's four states cannot say "let this through anyway" and cannot say "this no longer applies"
column: 5ea2db0272fc
state: ready
severity: major
priority: next
workstreams:
  - b3f924406e4c
links:
  - kind: relates_to
    to: 2f234e39172b
  - kind: relates_to
    to: 5ef07a3b83a3
---
A checklist item has four states, `pending`, `resolved`, `verified` and `failed`, and two things a workbench genuinely needs to say do not fit any of them. Both are here rather than on two cards, because they are one question about what the state set is missing and answering either alone would settle the shape of the other by accident.

## The first: let this through anyway

The operator ruled on 2026-09-10, answering a question on dinah-450, that a failed acceptance criterion holds a card rather than releasing it, and that a new state carries the permission to proceed regardless.

He rejected both options he was offered and supplied this instead, and the reason is worth keeping. The old behaviour made `failed` do two jobs at once: record that somebody checked and the work did not hold, and grant permission to carry on. Those are different facts and only one of them belongs to the person who ran the check. Splitting them lets `failed` mean what it says.

## The second: this no longer applies

Found on 2026-09-14 by Agent Design Review on dinah-449, the card that moves this project's development onto Dinah. The Andoneer board this project runs on can mark an item as no longer relevant, and Dinah cannot. That is not a cosmetic gap on the way across: an item that has stopped applying has no honest state here. Leaving it pending freezes its card at whatever column it names with nothing on the card explaining why, and every other state available asserts something that did not happen. `resolved` says somebody answered it, `verified` says somebody checked the work, `failed` says the work did not hold.

dinah-449 hit this with four such items of its own and worked around it by moving their content into the card's prose, which loses the structure and loses the hold. That workaround was written as temporary on the strength of this card existing, and this card did not cover it, so the promise had nothing behind it until now. It does not become permanent by being convenient.

Note the asymmetry with the first state, because it is the thing most likely to get blurred. "Let this through anyway" is a judgement that the work may proceed despite a real finding, and the finding stays true. "This no longer applies" says the question stopped being a question, usually because the card changed underneath it. A tool that offers one word for both invites people to record the first when they mean the second, which is exactly the conflation that made `failed` ambiguous in the first place.

## What exists today

Four states. dinah-450 landed the hold so that only `resolved` and `verified` release it, and dinah-484 gave a column's hold a direction, so an unsettled item can now hold a card on the way out of a station as well as on the way in. That makes both missing states more consequential than when this card was first filed: an item in the wrong state now pins a card in a place it cannot leave.

## What this card has to settle, and none of it is obvious

**The names.** `override` is what the operator called the first one in conversation and it is not necessarily the right word in the tool, because the move already has an override marker meaning something adjacent but different: momentary, per move, lifting capacity and loop limits too. Two things called override that behave differently is how a vocabulary starts lying. The second state needs a name that cannot be read as either "resolved" or "abandoned by the person who should have answered". Consider what somebody running a workbench with no code would call each.

**The verbs.** Six exist (file, cite, resolve, verify, fail, reopen) and none expresses either state. Decide for each whether it is a new verb or a flag on an existing one, and note that `reopen` already moves an item backwards, so the whole set defines what a state machine over six states permits. Say which transitions are legal and which are refused, including whether an item that no longer applies can come back.

**Whether a reason is required, and recorded.** The move-level override already demands one. A state that lets failed work through without a recorded reason is a silent way past a check, which is the failure this mechanism exists to prevent. The same argument applies with less force to the second state and should be made rather than assumed. The card should have to argue against requiring a reason rather than for it.

**Who may set each, and here the honest answer is uncomfortable.** Dinah records who an item is for, and dinah-484 has since made an operator-owned item settleable only by the operator, so the enforcement that did not exist when this card was filed now partly does. Establish what is actually true at the time you work this rather than inheriting either sentence, and say plainly what is and is not enforceable.

## What it touches beyond the states themselves

Every reader that switches on state has to handle both new values. The format document defines the state set and must gain them. The published profile gains whatever statements describe them, and that amendment belongs to this card, because dinah-450's own amendment was deliberately ratified as-is rather than extended to describe behaviour nobody had built. Note that dinah-480 already has open findings against that same conformance table, so coordinate rather than editing it blind.

## Urgency

The first state is not urgent. A criterion that genuinely failed and that the operator has decided to accept anyway is still passable using the move-level override, so nothing is stuck.

The second one is on the cutover's path. dinah-449 carries four items it cannot move across honestly, and every card that crosses with a stale item carries the same problem. Until this lands, that content sits in prose where nothing holds it and nothing counts it.
