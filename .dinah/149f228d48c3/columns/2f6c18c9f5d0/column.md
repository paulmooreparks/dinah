---
title: Design Queue
slug: design-queue
kind: work
operator_owned: false
---
The Design station's ready buffer: triaged cards waiting for spec-stage attention. Raw capture lives upstream in Intake; by the time a card sits here it has been through Triage and carries a tier, a lane, and the triage decision that it deserves design effort.

### Who pulls, and where the card goes

Spec-stage agents and operators. Entry into this queue is already the go signal from Triage, so agents may pull freely: highest priority first, tier-eligible cards only.

**A pull from here promotes the card into Spec/Ready**, which is the next stage on every lane that passes through this queue. Nothing else happens in this column.

### When to leave a card here

- Tier-mismatched with the sessions currently running.
- Deliberately parked behind an upstream dependency (see incoming `parked_behind` links).
- The Design station's WIP is full; the queue absorbs the wait so the station does not.

Queue age is the station's demand signal: if this queue grows while Spec sits idle, dispatch capacity is missing rather than work being stuck. There is no WIP cap on the queue itself; caps apply downstream at the station's flow columns.

### A card that does not need a spec should not be here

Triage routes work needing a contract to this queue and mechanical work straight to Build Queue. If you pull a card whose contract is already obvious from the card alone, the routing call was wrong: move it to Build Queue and say why, rather than writing a spec nobody needed.
