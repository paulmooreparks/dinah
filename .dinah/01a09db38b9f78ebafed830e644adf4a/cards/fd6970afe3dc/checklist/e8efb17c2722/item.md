---
kind: acceptance_criterion
state: pending
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:19Z
ordinal: 4
---
If the chosen response adds a CORE-MOVE refusal: dinah help move's printed table carries the new row at the position the evaluation order assigns it, cmd/dinah/main_test.go's ratifiedMoveRefusalTable golden string is updated to match, and a run against the old, un-updated golden string fails (proving the golden comparison is actually exercised). Not applicable, and satisfied vacuously with a one-line note saying so, if the chosen response adds no refusal.