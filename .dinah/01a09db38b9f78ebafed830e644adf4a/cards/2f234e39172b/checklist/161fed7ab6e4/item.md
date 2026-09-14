---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:35Z
ordinal: 4
note: "test: cmd/dinah/exithold_test.go#TestAColumnHoldingOnTheWayOutRefusesTheDeparture (a column declaring hold: out, an acceptance_criterion filed with --column doing by SLUG exercising dinah-473's resolution, the forward move refused with exit code 2 and refusal name dinah.unresolved-item-exit naming the item, then `dinah verify` and the same move succeeding). Also run against the built binary: `dinah move pf-1 review` answered `dinah.unresolved-item-exit this card carries the item 3c9570ac7fd3, which is not resolved, and this column holds until it is; answer it, then resolve, verify or fail that item` with exit=2, and after `dinah verify pf-1/criteria/1` the same move exited 0. observed before: fail, after: pass. Armed by relocating the canLand block below the three destination rows, which reddened exithold_test.go:219; restored byte-identically."
---
**End-to-end, run against the built binary:** create a column declaring `hold: out`; file an `acceptance_criterion` naming that column via `--column` (its slug, exercising dinah-473's own resolution); claim a card there and attempt `dinah move <card> <a later column>`; confirm it is refused `dinah.unresolved-item-exit` naming the item. `dinah verify` the item, then repeat the same move and confirm it succeeds.