---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:17:13Z
ordinal: 6
note: go test ./cmd/dinah/... -run TestEveryRowStartsItsColumnsAtOneDisplayColumn green (19.77s) at merged commit 1648be0.
---
cmd/dinah's row-sweep suite predicts the corrected behaviour rather than the old one, with sweptStates rewritten to mirror axisValueOrder's two rules (declared states drawn unconditionally except blocked, which needs occupancy; undeclared states drawn only when actually held). Test hook: go test ./cmd/dinah/... -run TestEveryRowStartsItsColumnsAtOneDisplayColumn green.