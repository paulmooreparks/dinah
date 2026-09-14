---
kind: decision
state: resolved
ts: 2026-09-14T02:18:04Z
ordinal: 20
note: "Four things they need already stand there: runCLI and newBench for the behaviour probes, parseReferencesGuideTable at references_command_resolution_test.go:161 for the table, repositoryRoot at guide_guard_test.go:38 for the tree walk, and displayWidth at row.go:59 for the width check. The prose figure ledger's derivation map is in that package too, so commandsTakingAReference is written once and the ledger, the roster test and the width test all call it. internal/guide could import internal/verb without a cycle, but it would carry a second copy of the parser and no displayWidth at all."
---
All six new tests live in cmd/dinah rather than internal/guide.