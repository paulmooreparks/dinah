---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:28Z
ordinal: 20
note: "`runMCP` (`cmd/dinah/commands.go:1642-1697`) reads `--root`/`DINAH_MCP_ROOT` into `s.mcpRoot`, which bounds which workbenches may be named, and separately opens the session's library through the workbench ladder, which becomes the default for a call naming no workbench. `TestMCPKeepsDiscoveredDefaultButStopsNarrowingWithNoRoot` in `cmd/dinah/mcp_startup_test.go` pins that a server with a default but no root still answers a call naming a different workbench by absolute path. So `--workbench` alone gives the reader's agent their workbench for free and fences it out of nothing, while `--root` would silently narrow what their agent can reach. `cmd/dinah/main.go:131-134` puts SourceFlag above SourceEnvironment, so the flag also outranks a `DINAH_WORKBENCH` the extension host happens to carry."
---
The published argv is `--workbench <root> mcp`, and carries no `--root`.