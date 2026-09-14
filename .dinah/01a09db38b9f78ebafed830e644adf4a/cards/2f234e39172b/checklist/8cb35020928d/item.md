---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:37Z
ordinal: 21
note: Yes, symmetrically. The check added to `canLand` reads only `departure.HoldsOnExit()` and whether an unsettled item names the departure; nothing in it reads `forward`. This matches the existing entry hold, which already refuses a backward move INTO a gated column exactly as it refuses a forward one (today's `destination.GateItems`/soon `destination.HoldsOnEntry()` block carries no forward check either). A push-back out of a review station with an unanswered question stays with the card in either direction.
---
Does an exit hold refuse a backward move out of the column it names, as well as a forward one?