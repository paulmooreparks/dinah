---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:48Z
ordinal: 1
note: "PASS. `dinah help mcp` prints the syntax line `mcp [--root <dir>]` and the arguments row `[--root <dir>]`. Both are generated from one source: the syntax line via verb.Usage -> Tokens -> params[\"mcp\"] -> Param.Token(), the row via verb.Params(name) -> param.Token(). Declared at internal/verb/definition.go:384. Same Token() feeds both, so the line and the row cannot diverge. Verified against a binary built from origin/main at 2b9f2d7."
---
`dinah help mcp` prints the syntax line `mcp [--root <dir>]` and an arguments row whose left cell is `[--root <dir>]`.