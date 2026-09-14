---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:29Z
ordinal: 1
note: "Re-run in Test: go test ./cmd/dinah/... -run TestEveryFieldOfEveryKindRoundTripsAtBothHeads and ./internal/verb/... -run TestEveryGuardedFieldRefusesAndAcceptsAtItsOwnGate both pass with column/hold present."
---
bench.FieldsOf("column") includes "hold", and bench.FieldOf("column", "hold") reports Key="gate_items", Guard=GuardHold, Clearable=false.