---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:28Z
ordinal: 12
note: Write-time validation removes every path by which a write could turn a live gate dead without the caller having typed the value responsible (a clear is explicit, a reassignment is explicit and self-reports via response.Detail, and an unresolvable value is now refused outright). The remaining case is an item already stale on disk from before this card, which dinah check's new check.item-column-unresolved finding covers rather than a write-time report.
---
No new reporting mechanism is added for a "silently lifted hold."