---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:30Z
ordinal: 7
note: "Re-run: go test -run TestTheHoldJournalsInTheStoredForm passes (field: \"hold\", from/to in stored form true/\"\")."
---
A successful `dinah set <column> hold on|off` appends exactly one column_updated journal event carrying field: "hold" and from/to values in the stored form ("" / "true"), not on/off.