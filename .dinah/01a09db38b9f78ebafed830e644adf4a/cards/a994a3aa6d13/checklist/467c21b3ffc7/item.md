---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:48Z
ordinal: 2
note: "PASS. The refusal table prints two rows, dinah.unknown-root then dinah.outside-root, and no others. Order matches the declaration in beyondChecks[\"mcp\"] at internal/verb/checks.go:323-326. Nothing is prepended, because mcp reaches checkLists through beyondChecks rather than runsWorkbenchChecks. Layout wart noted separately and not counted against this criterion: row 1's middle cell overruns its column and pushes dinah.unknown-root onto a continuation line. AC-2 says nothing about line placement, but see the AC-26 finding, where the accepted sketch argues explicitly for the fit this breaks."
---
`dinah help mcp` prints a refusal table whose first row names `dinah.unknown-root` and whose second names `dinah.outside-root`, in that order and no other.