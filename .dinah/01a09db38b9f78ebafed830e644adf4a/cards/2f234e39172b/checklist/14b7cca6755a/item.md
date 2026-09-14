---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:35Z
ordinal: 5
note: "test: cmd/dinah/exithold_test.go#TestAColumnHoldingOnTheWayOutRefusesTheDeparture, the `back := runCLI(t, root, \"move\", \"fx-1\", \"intake\")` half, which asserts exit code 2 and refusal name dinah.unresolved-item-exit naming the item on a regressive departure. Also run against the built binary: `dinah move pf-1 intake` answered `dinah.unresolved-item-exit this card carries the item 3c9570ac7fd3, which is not resolved, and this column holds until it is` with exit=2, identical to the forward case. observed before: fail, after: pass. Armed by adding `forward &&` to the canLand condition, which reddened exithold_test.go:75 with `the regressive departure exited 0, wanted 2` while the forward half stayed green; restored byte-identically."
---
**End-to-end, run against the built binary, the same card as AC-4 but attempting the regressive direction first:** with the item still unresolved, attempt `dinah move <card> <an earlier column>` (a backward move) out of the `hold: out` column; confirm it is also refused `dinah.unresolved-item-exit`, proving the hold is not forward-only.