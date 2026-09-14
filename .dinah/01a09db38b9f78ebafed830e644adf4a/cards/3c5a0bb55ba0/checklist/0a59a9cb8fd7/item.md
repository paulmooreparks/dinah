---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:55Z
ordinal: 15
note: "Eleven of the fifteen commands resolve through ResolveEntity, so putting the refusal there reaches all eleven from one edit and cannot be forgotten by a twelfth caller added later. The three commands that must accept a collection, plus the two that must refuse it without going through ResolveEntity, ask ResolveReference themselves and branch. The alternative considered was a separate ResolveCollection that every one of the fifteen calls first, which walks the tree twice on every reference and leaves the refusal as a line each caller can omit. The cost of the shape chosen is that a future caller of ResolveEntity gets the refusal whether it wants it or not, which is the right default: a command that takes one entity is exactly the caller of that resolver."
---
One resolver answers both questions. Bench.ResolveReference returns either an entity or a collection, and Bench.ResolveEntity becomes a reading of it that refuses dinah.is-a-collection.