---
title: Build Queue
kind: work
operator_owned: false
---
Parking-lot for cards that have cleared Design, including any operator ruling or approval raised along the way, but are not yet claimed by an implementer. The BUILD station's ready buffer, absorbing priority shuffling between review and implementation.

Cards here are first in line for a pull. Capability-matched: a session running at workhorse pulls workhorse cards, a minimal session pulls minimal cards, and a higher tier can pull anything it is eligible for.

### Who pulls, and where the card goes

Any session whose `tier_class` satisfies the card's `expected_tier`, highest priority first. There is no subagent directive on this column itself; **a pull from here promotes the card into Implement/Ready**, where `card-implement` runs.

Cards reach this queue two ways. Most arrive from Agent Review, or from Operator Review once the operator has accepted whatever the card produced. Some arrive straight from Triage, on the Fix lane, with no spec at all because their contract is obvious from the card alone. Both are legitimate, and the Implement column knows the difference.

### Check the gates before you claim

A card here may be waiting on another card. A `blocks` or `parked_behind` link materializes its gate on the gated card's next intake step resolved along that card's own lane, which is frequently this column but is not guaranteed to be, so the placement is not something to infer from where the card is sitting. Run `evaluate_gates` and read the answer: skip a card reporting `gated`, and treat one reporting `bypassed` as pullable, since that status means the gate's column sits behind the card and the event it constrains can no longer occur.

### When to leave a card here

- Tier-mismatched with current sessions, waiting for a capable implementer to arrive.
- Higher-priority cards in flight are consuming WIP capacity at Implement.
- Deliberately parked behind an upstream dependency (see incoming `parked_behind` links).

There is no WIP cap on the queue itself; the cap applies downstream at Implement.

### A card must never sit here waiting on the operator

Work waiting on a person does not belong in a queue that advertises itself as ready to build, because the next agent along will pull it and find it cannot proceed. `block_card` needs a flow column and an active claim, and this is a pull queue, so the repair always starts by getting the card out of here. Which way depends on what it is waiting for.

**Waiting on an unexpected ruling**, meaning a question nobody anticipated: move it back to Agent Review, claim it, and block it there with `kind="operator_decision"` and the question as the reason. That block clears the claim you just took, and nothing follows it: no `move_card`, no `release_card`.

**Waiting on an expected approval**, meaning the card produced a UX sketch, an interface, a flow change or published copy that the operator has to accept: move it to Operator Review and name, in the move-note, the acceptance criterion the operator's approval will create. That is not a block and the card is not stuck; it simply reached a checkpoint early.

If you cannot tell which, the falsifier decides it: if you can write the acceptance criterion approval would create, it is the second case. If you cannot, it is the first.
