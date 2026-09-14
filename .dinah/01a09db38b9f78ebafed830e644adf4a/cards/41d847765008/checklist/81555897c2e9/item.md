---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:17:12Z
ordinal: 1
note: "Re-verified at merged commit 1648be0 (branch merged with origin/main, pushed). go test ./internal/bench/... -run TestStatesAndHoldsStateAgree green, all 5 subtests. Manual fixture confirms: Column.States() returns nil for intake/queue(dinah.buffer)/outside(awaiting_outside)/done, {ready,active,blocked} for doing."
---
Column.States() returns nil for every column TakesWorkUp() answers false for (intake, done, dinah.buffer, or AwaitingOutside: true), and {ready, active, blocked} unchanged otherwise. Test hook: go test ./internal/bench/... -run TestStatesAndHoldsStateAgree, after rewriting that table's queue-kind expectations to an empty slice.