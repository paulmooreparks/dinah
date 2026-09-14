---
kind: decision
state: resolved
column: 4b38abe7ebd5
owner: holder
ts: 2026-09-14T02:18:08Z
ordinal: 30
note: "The spec produced its list by grepping for \"workstream\" and adding the one \"attachment\" literal it had read. AC-9's own grep is wider, and running it at 22a35fc named fourteen entity-kind literals rather than seven: internal/bench/entity.go:383 and internal/verb/beyond.go at 190, 230, 275, 639, 652, 675, 686, 697, 710, 724, 736 (twice, \"card\" and \"workstream\"), 745 and 748. The extra ones are the \"workbench\", \"column\", \"card\" and second \"attachment\" comparisons the narrower grep could not see. Converting only the seven would have left AC-9 failing on its own terms, so all fourteen now read bench.Kind* constants. The change is mechanical, the compiler checks it, and it touches no behaviour: after it the first grep returns seven hits and none of them is an entity kind."
---
Every entity-kind string literal in production code was converted, not the seven sites the spec's section 5 enumerated.