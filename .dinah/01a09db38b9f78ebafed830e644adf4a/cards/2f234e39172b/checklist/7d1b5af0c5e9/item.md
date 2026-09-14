---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:36Z
ordinal: 20
note: Yes, via the fourth value, `both`, on the same one-field enumeration as the other three (`off`, `on`, `out`, `both`), not via two independent booleans. `HoldsOnEntry()` and `HoldsOnExit()` each read `Column.Hold` against their own pair of legal values (`on`/`both` and `out`/`both` respectively), and each check reads `bench.GatingItems` against its own column ID (destination for entry, departure for exit) at its own point in a move; the two never interact, so declaring `both` is meaningful (refuse entry until settled, separately refuse to leave until settled) rather than contradictory. This supersedes the prior revision's two-independent-booleans answer to the same question.
---
May a column hold in both directions at once, and what does that mean?