---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:17:40Z
ordinal: 5
note: "Armed. Red: \"testdata\\prose-figures.txt:60: the entry expects the figure Five beside commands at docs/quick-start.md:562, and that line carries no such figure\". Line 562 does carry a figure and a noun, five and verbs, so the check compared the pair rather than merely finding something. Restored, green."
---
A stale ledger entry is caught. Arming: repoint the entry for `docs/quick-start.md:561` at line 562, run `go test ./cmd/dinah -run TestNoProseFigureEntryIsStale`, and watch it fail naming the entry, line 562, and the figure and noun it expected to find there. Restore the line number and watch the run go green.