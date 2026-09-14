---
kind: decision
state: resolved
column: 4b38abe7ebd5
owner: holder
ts: 2026-09-14T02:18:38Z
ordinal: 33
note: "The destination, not the departure. A departure declaring operator_owned is refused NotOperator by canRoute (internal/verb/mutate.go:343), which pull() calls before claimableState, pullableDeparture and canLand, so on AC-16's literal fixture canLand is never reached and no ordering inside it can be observed. Pull's row 13 is operatorReservesClaim(destination, ...) inside canLand, which is the row AC-16's own sentence names (\"the operator-owned reservation, now row 13\"). The fixture therefore declares hold: out on the departure and operator_owned on the destination, which reaches the pair AC-16 intends and proves exactly what it claims. Run in cmd/dinah/exithold_test.go#TestTheExitHoldAnswersAheadOfTheOperatorReservedClaimOnAPull."
---
AC-16 says to declare operator_owned on the departure column so that NotOperator would otherwise win the race. Which column actually carries it in the fixture?