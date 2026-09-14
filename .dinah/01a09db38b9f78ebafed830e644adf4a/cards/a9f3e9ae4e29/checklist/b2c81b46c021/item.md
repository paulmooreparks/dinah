---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:29Z
ordinal: 6
note: "Re-run: go test -run TestTheHoldIsTheOperatorsToTurn passes. Also exercised live as required: set doing hold off --actor someoneelse refused not-operator exit 2; same call --actor prober (operator) exit 0; get doing hold --actor someoneelse succeeds exit 0 answering \"on\"."
---
`dinah set <column> hold on` run by a non-operator actor refuses not-operator (exit 2) and changes nothing; the same call as the operator succeeds. `dinah get <column> hold` run by a non-operator actor succeeds.