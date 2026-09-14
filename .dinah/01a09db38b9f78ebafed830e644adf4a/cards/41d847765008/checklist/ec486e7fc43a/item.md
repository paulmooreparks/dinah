---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:17:12Z
ordinal: 3
note: "go test ./internal/verb/... -run TestAColumnThatTakesNoWorkUpDrawsNoStateGroupWhenEmpty green (both subtests: intake/buffer/done, and awaiting-outside). Manual fixture reproduces this at the command: hand-built workbench with intake, queue(dinah.buffer), doing, outside(awaiting_outside:true), done columns; emptying intake/queue/outside/done draws zero children under each in `dinah tree --group-by column,state`."
---
A tree grouped by column then state draws no state group at all under a column with no card standing in it and no owner taking work up there (intake, dinah.buffer, or done alike). Test hook: go test ./internal/verb/... -run TestAColumnThatTakesNoWorkUpDrawsNoStateGroupWhenEmpty, new test specified in the spec over the existing buffer harness.