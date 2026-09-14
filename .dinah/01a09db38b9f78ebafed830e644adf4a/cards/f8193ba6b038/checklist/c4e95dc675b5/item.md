---
kind: decision
state: resolved
ts: 2026-09-14T02:16:31Z
ordinal: 13
note: internal/mcp/mcp.go encodes answers with json.MarshalIndent directly and never touches session or s.format. docs/design/surfaces.md states the MCP surface is a thin mapping of the canonical JSON; giving MCP its own compact projection changes what that sentence says and needs its own design pass, including whether an LLM client sees the same token economics a shell-scripted driver loop does, since an MCP tool result already rides inside a JSON-RPC envelope. A later card can extend compactEncode to an MCP-side caller without touching anything this card defines.
---
The MCP head is out of scope for this card and continues to answer every tool call with canonical JSON only, whatever DINAH_FORMAT says.