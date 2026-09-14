---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:25Z
ordinal: 22
note: "Measured at 65a80ad over 93 non-test Go files: 140 literal key asks, 10 fires, all 10 false. Seven are prefix concatenations in `cmd/dinah/help.go`, `reshape.go` and `table.go`; three are `Has` on `internal/bench/frontmatter.go`'s own type, which shares the method name and is no renderer. Beyond them sit 24 distinct dynamic key sites across four packages, each needing a family declaration before the sweep could be honest, against the extension's one. Somebody would pick that up on its own, so it is a card rather than a note, and I have deliberately not filed one: the operator's queue is what he is trying to drain and the decision is his. The reverse placeholder direction is the opposite case, three lines added to an existing Go test, 6,559 pairs compared and 0 fires, so it ships here."
---
A code-to-catalogue key sweep over the CLI is declined; the CLI's missing placeholder direction is included.