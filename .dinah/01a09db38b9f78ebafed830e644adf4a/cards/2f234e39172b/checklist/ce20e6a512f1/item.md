---
kind: decision
state: resolved
column: 4b38abe7ebd5
owner: holder
ts: 2026-09-14T02:18:38Z
ordinal: 36
note: By the suite's own regeneration flag, `go test ./cmd/dinah/ -run TestTheQuickStartMatchesTheTool -update-quick-start`, then reading the diff and committing it. The counts move from 937 to 941 in all eight rows. The four keys are check.move.11, check.pull.12, refusal.dinah.unresolved-item-exit and refusal.dinah.unresolved-item-exit.next; the renumbered check.pull rows are renames rather than additions and cost nothing.
---
Four new base catalog keys change what `dinah version --catalogs` prints, which docs/quick-start.md pins as a replayed transcript. How was that fixture updated?