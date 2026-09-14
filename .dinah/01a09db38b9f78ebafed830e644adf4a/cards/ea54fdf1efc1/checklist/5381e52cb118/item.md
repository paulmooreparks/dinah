---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:11Z
ordinal: 29
note: "`doCard` at `internal/mcp/tools.go:730` wraps with `[]string{\"show\", \"log\", \"card\"}`, and copying that into `readField` would have published an affordance naming `card`, the tool this same card deletes. It is also wrong on its own terms for a tool answering a workbench, a column, a workstream, a comment, an item or an attachment, where `show` and `log` do not apply. `readAffordances` at `:528` is what every other bench-level read wraps with and every name in it is served. `set_field` returns `l.SetField(r)` and a library response carries its own affordances, which `internal/mcp/mcp.go:370` translates through `surfaceAffordances` before serving, so no literal is written there at all. The check is minted because nothing in the tree held this direction: the existing assertions check presence and check that an affordance is not a command spelling, and an affordance naming a deleted tool passes both. Reading the served set from a live `tools/list` call rather than from a second hand list is what keeps the new check from being one list compared against another."
---
`readField` publishes `readAffordances` rather than the bespoke literal `doCard` uses, `set_field` publishes no literal at all, and AC-11 mints the check that no affordance names a tool this head does not serve.