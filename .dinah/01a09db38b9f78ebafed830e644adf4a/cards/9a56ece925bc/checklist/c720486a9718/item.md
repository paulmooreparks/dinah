---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:18Z
ordinal: 18
note: "Both instances of this defect, the collection one on dinah-455 and this one, were found by looking at one shape, so the answer to this card is a guard that enumerates rather than a second hand-written list. `editReferenceShapes` walks `bench.Contains` recursively and emits the eight shapes declared outside that table, each with a comment naming where the resolver declares it, so a mount added later produces new shapes with no edit to the test. The predicate that decides which collection refuses `dinah.unknown-path` rather than `dinah.is-a-collection` is renamed to `bench.AddressedInItsOwnRight` and exported for the same reason: a test spelling out `card` and `column` for itself would be a second copy of a rule `descend` already declares, and the two would drift."
---
The sweep generates its reference shapes from the containment table, and `addressedInItsOwnRight` is exported so it can read the rule rather than restate it.