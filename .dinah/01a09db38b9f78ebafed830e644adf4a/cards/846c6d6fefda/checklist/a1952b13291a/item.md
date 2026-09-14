---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:17:10Z
ordinal: 1
note: "Test: TestEnumerateFindsTheWorkbenchInTheRootsOwnContainer, internal/bench/rootenumeration_test.go. Rerun: `go test ./internal/bench/ -run TestEnumerateFindsTheWorkbenchInTheRootsOwnContainer -v`. Green after the fix. Armed twice, both red: (a) against today's unfixed code before the fix landed, reporting \"answered 0 candidates ... []\"; (b) with the benchIn(root, false) probe block deleted from enumerate in internal/bench/bench.go, same failure. Assertion that goes red: rootenumeration_test.go:67 (the count) and :70 (listed[0].Path != anchor, so a candidate answered for the wrong reason does not pass)."
---
A unit test in internal/bench builds a directory whose own .dinah holds exactly one workbench (bench.Instantiate into <root>/.dinah/<id>, mirroring Init's own layout) and asserts enumerate(root)/Enumerate(root) returns exactly one Candidate whose Path is that anchor directory. Must fail against today's code (already confirmed by the spec's probe run) and must fail again if enumerate's added benchIn(root, false) probe is reverted.