---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:17:46Z
ordinal: 13
note: "This is already possible today, not deferred to a future lanes card: canRoute admits any declared column as a move destination, not only the flow's next one, so a card can already jump past a gated column. The gate simply never fires for that card, the same as any other queue or inspection point a jump-move skips. No code is needed to special-case it."
---
What happens to a gate naming a column a particular card's route never reaches?