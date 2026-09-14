---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:52Z
ordinal: 21
note: dinah-456 section 4.3 rules that contents adopts show's aliased spelling. Two producers composing one spelling is how they came to disagree in the first place, so the walk calls the same function rather than copying its rule. The walk needs the item's kind and its position within that kind, which it reads with itemKindAt and a per-mount counter; both counts run over containmentMembersOf, which sorts through bench.SortByOrdinal exactly as bench.Items does, so the two agree by construction rather than by inspection.
---
One composer, verb.itemRef, produces an item's reference for both Library.Show and the containment walk.