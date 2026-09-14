---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:32Z
ordinal: 18
note: "Both registered: cmd/dinah/commands.go's table gains {name: \"link\", run: runLink, bounded: 3} and the unlink twin, and internal/mcp/tools.go's roster gains link_card and unlink_card. `go test ./cmd/dinah/... ./internal/mcp/...` passes, which is TestEveryLibraryCommandIsDispatchedOrExempted and its MCP twin. The MCP side needed three further edits the roster entry alone would not have satisfied and which its own guards caught: assignValue gained the \"to\" case (TestEveryDeclaredParameterReachesItsDeclaredField), publishedProperties gained both rows, and scripts/verb_selection_fixture.json gained a scenario each."
---
`link` and `unlink` are each registered in `cmd/dinah/commands.go`'s command table and `internal/mcp/tools.go`'s roster; `go test ./cmd/dinah/... ./internal/mcp/...` passes, which is what `TestEveryLibraryCommandIsDispatchedOrExempted` and its MCP twin enforce.