---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:30Z
ordinal: 8
note: "Re-run: go test -run TestEveryFieldOfEveryKindRoundTripsAtBothHeads passes, including the protocol:_column/hold subtest, confirming MCP get_field/set_field parity with no MCP-specific code."
---
The MCP tools get_field/set_field (internal/mcp/tools.go) produce the same behavior as the CLI verbs for the "hold" field on a column reference, with no MCP-specific code added beyond what the AllFields()-derived schema already threads through.