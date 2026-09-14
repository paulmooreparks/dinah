---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:28Z
ordinal: 10
note: "Re-verified live: in the same fixture, a third checklist item carrying no column key at all produced zero findings (only the 2 hand-edited items were reported, \"2 defects.\"). Also confirmed via go test ./internal/bench/ -run TestCheckReportsEveryItemColumnThatCannotHoldACard -v PASS, which pins the exact-set assertion covering this case."
---
A card with no checklist items, and a card whose items all carry an empty column, both produce zero FindingItemColumnUnresolved findings from bench.Check(), proving the sweep does not false-positive on an absent value.