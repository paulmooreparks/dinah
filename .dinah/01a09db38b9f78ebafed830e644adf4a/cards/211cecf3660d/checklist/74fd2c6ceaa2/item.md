---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:27Z
ordinal: 6
note: "Re-verified via go test ./internal/bench/ -run TestCheckReportsEveryItemColumnThatCannotHoldACard -v: PASS. Also reproduced live: a card with two hand-edited items (one to a nonexistent column, one to a real column's slug rather than identifier) and a third item left with no column key produced exactly 2 findings from `dinah check`, matching only the two bad items."
---
bench.Check() against a fixture workbench with one card whose item's column is hand-written to an unresolvable value, a second card whose item's column is hand-written to a real column's slug (not its identifier), and a third card whose item correctly carries a resolved identifier, reports FindingItemColumnUnresolved for exactly the first two items and no others; assert the exact set found, not "at least these two."