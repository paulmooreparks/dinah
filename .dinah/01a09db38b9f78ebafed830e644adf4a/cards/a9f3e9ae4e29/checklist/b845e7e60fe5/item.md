---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:29Z
ordinal: 4
note: "Re-run: go test -run TestTheHoldWrittenTwiceJournalsOnce passes."
---
`dinah set <column> hold on` run twice against the same column: the second call succeeds (exit 0, outcome ok) and appends no journal event (compare the column's journal file line count before/after the second call).