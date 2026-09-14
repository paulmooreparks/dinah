---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:17:10Z
ordinal: 5
note: "Test: TestWorkbenchesListsTheWorkbenchInTheRootsOwnContainer, internal/mcp/mcp_test.go (verb.Init writes the real .dinah container, askUnderRoot serves the workbenches tool bounded by the directory whose .dinah holds it). Rerun: `go test ./internal/mcp/ -run TestWorkbenchesListsTheWorkbenchInTheRootsOwnContainer -v`. Green after the fix. Armed twice, both red at mcp_test.go:1240 reporting `listed []`: (a) against today's unfixed code before the fix landed; (b) with the benchIn(root, false) probe deleted from enumerate. This closes the card's \"suspected second instance\" on the same fix, with no second card needed."
---
A test in internal/mcp/mcp_test.go builds a workbench via verb.Init (real .dinah container) and asserts a workbenches tool call served with askUnderRoot against the directory whose .dinah holds it lists that workbench. Must fail against today's code (the spec's probe already shows the underlying call returns empty for this layout).