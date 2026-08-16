---
title: Build Queue
kind: work
operator_owned: false
---
Parking-lot for cards that have cleared the design half, including the operator's own station there, but are not yet claimed by an implementer. The BUILD station's ready buffer, absorbing priority shuffling between review and implementation.

Cards here are first in line for a pull. Capability-matched: a session running at workhorse pulls workhorse cards, a minimal session pulls minimal cards, and a higher tier can pull anything it is eligible for.

### Who pulls, and where the card goes

Any session whose `tier_class` satisfies the card's `expected_tier`, highest priority first. There is no subagent directive on this column itself; **a pull from here promotes the card into Implement/Ready**, where `card-implement` runs.

Cards reach this queue two ways. Most arrive from Operator Design Review, the operator's station that closes the design half. Some arrive straight from Triage, on the Fix lane, with no spec at all because their contract is obvious from the card alone. Both are legitimate, and the Implement column knows the difference.

### Check the gates before you claim

A card here may be waiting on another card. A `blocks` or `parked_behind` link materializes its gate on the gated card's next intake step resolved along that card's own lane, which is frequently this column but is not guaranteed to be, so the placement is not something to infer from where the card is sitting. Run `evaluate_gates` and read the answer: skip a card reporting `gated`, and treat one reporting `bypassed` as pullable, since that status means the gate's column sits behind the card and the event it constrains can no longer occur.

### When to leave a card here

- Tier-mismatched with current sessions, waiting for a capable implementer to arrive.
- Higher-priority cards in flight are consuming WIP capacity at Implement.
- Deliberately parked behind an upstream dependency (see incoming `parked_behind` links).

There is no WIP cap on the queue itself; the cap applies downstream at Implement.

### A card must never sit here waiting on the operator

Work waiting on a person does not belong in a queue that advertises itself as ready to build, because the next agent along will pull it and find it cannot proceed. A card in this queue has already passed the design half's operator station, so a question surfacing now is one of two kinds, and the falsifier is whether implementation can start without the answer.

**Implementation can proceed despite the question:** file it as an open question with `owner="operator"`, the recommendation and tradeoffs in the note, and leave the card here. It rides forward with the card and gets answered at Operator Code Review, the build half's station, on lanes that run it; on the Fix lane the implementer blocks instead when the question turns real.

**Implementation cannot start without the answer:** the card is not ready to build, whatever column it sits in. Move it back to Agent Design Review, claim it, and block it there with `kind="operator_decision"` and the question as the reason. That block clears the claim you just took, and nothing follows it: no `move_card`, no `release_card`.
