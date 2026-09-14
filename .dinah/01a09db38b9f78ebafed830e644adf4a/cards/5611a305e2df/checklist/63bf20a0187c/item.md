---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:17:40Z
ordinal: 1
note: Re-checked independently by Test on 56db064. `go test ./cmd/dinah -run 'TestEveryProseFigureIsDeclared|TestNoProseFigureEntryIsStale|TestEveryDerivedProseFigureMatchesTheBinary|TestEveryCountedSetIsCountedConsistently|TestEveryLedgerReferenceIsLive'` exits 0 on an unmodified tree, and cmd/dinah/testdata/prose-figures.txt carries exactly nine `noun ` lines and twenty-three `figure=` entries.
---
The prose figure ledger and its five checks are green on an unmodified tree. Verdict: `go test ./cmd/dinah -run 'TestEveryProseFigureIsDeclared|TestNoProseFigureEntryIsStale|TestEveryDerivedProseFigureMatchesTheBinary|TestEveryCountedSetIsCountedConsistently|TestEveryLedgerReferenceIsLive'` exits 0, and `cmd/dinah/testdata/prose-figures.txt` carries nine noun lines and twenty-three figure entries.