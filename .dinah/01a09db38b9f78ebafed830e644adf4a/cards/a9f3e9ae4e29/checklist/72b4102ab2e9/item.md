---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:29Z
ordinal: 5
note: "Re-run: go test -run TestTheHoldRefusesAnyValueButOnAndOff passes (4 subtests: empty, maybe, true, On). Also confirmed live: set doing hold maybe exits 2 with \"malformed hold is missing, empty, or will not parse ...\"."
---
`dinah set <column> hold` with no value, and `dinah set <column> hold maybe` (any value other than exactly on/off), both refuse malformed (exit 2) naming "hold" as the detail, and change nothing on disk.