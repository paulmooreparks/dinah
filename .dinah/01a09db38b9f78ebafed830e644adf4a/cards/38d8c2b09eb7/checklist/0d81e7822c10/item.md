---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:38Z
ordinal: 6
note: Findings 5 and 6, verified against the shipped binary's own guide output rather than against the source file, so a guide that failed to re-embed is caught too. The quick-start block named is replayed by TestTheQuickStartMatchesTheTool, which makes it the authority for the shape.
---
`dinah guide principles` run from a scratch directory under C:\dinah-scratch prints a columns table whose heading row carries the same column names in the same order as the heading row of the driven block at docs/quick-start.md:367, and `dinah guide query` prints no sentence containing the word "nine".