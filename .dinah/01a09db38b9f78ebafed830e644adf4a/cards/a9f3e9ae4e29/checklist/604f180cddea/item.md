---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:29Z
ordinal: 2
note: "Re-run: go test -run TestTheHoldWritesTheDeclarationAndReadsItBack passes. Also confirmed live against built binary in a throwaway workbench: set doing hold on exit 0, get doing hold answers on."
---
`dinah set <column> hold on` against a column not currently holding writes gate_items: true to that column's column.md and nothing else in its frontmatter changes; a subsequent `dinah get <column> hold` on the same column returns "on".