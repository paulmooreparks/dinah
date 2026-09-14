---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:48Z
ordinal: 3
note: "PASS. `dinah` with no arguments diffs byte for byte against cmd/dinah/testdata/help.txt (empty diff). The mcp line reads `mcp [--root <dir>]` with its summary, and the Environment line names DINAH_MCP_ROOT. One ambiguity recorded rather than resolved silently: the criterion quotes the summary lowercase as \"serve workbenches over MCP on stdio\" while both binary and fixture print \"Serve...\". The stated test is the byte-for-byte fixture comparison and it passes; the sentence-case change arrived after 12ea2f8, most likely from the help-formatting work."
---
`dinah` run with no arguments prints an `mcp` line reading `mcp [--root <dir>]` beside the summary `serve workbenches over MCP on stdio`, and an Environment line naming `DINAH_MCP_ROOT`.