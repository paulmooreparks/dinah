---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:17:33Z
ordinal: 28
note: No duplicate entry is written and no duplicate journal event is appended, mirroring SetCardTierAt's own "was == absolute, nothing written" precedent for a card-owned declared block.
---
D-7: link is idempotent on an identical existing (kind, to) pair.