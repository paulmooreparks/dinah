---
kind: acceptance_criterion
state: pending
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:19Z
ordinal: 2
---
legalMoves for a card standing at a non-terminal column, on a workbench declaring at least four columns, is asserted against the full column list: every column at a greater Position is Forward and every column at a lesser Position is Backward. If the chosen response changes what legalMoves offers for a column more than one step forward, the test asserts the new behavior explicitly (e.g., omitted or marked); if the response does not touch legalMoves, the test asserts today's behavior is unchanged. A run asserting neither, or asserting silence on the point, fails this criterion.