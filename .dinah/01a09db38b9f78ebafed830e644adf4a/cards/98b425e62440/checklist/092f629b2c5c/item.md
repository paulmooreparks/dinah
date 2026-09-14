---
kind: acceptance_criterion
state: verified
column: b69abf918c42
owner: holder
ts: 2026-09-14T02:16:35Z
ordinal: 11
note: Confirmed PR 123 (dinah-31) already landed on trunk (commit 3e855ce, present in origin/main). The merge into this branch carried "format" correctly into sessionFlagNames and into the roster in TestParseArgsRecordsNoDomainCaptureForASessionFlag (verified via grep and the passing "format" subtest above). go test ./cmd/dinah/ green on the merged result.
---
At Merge, check whether pull request 123 (dinah-31) has already landed. If it has, trunk carries `format` in `sessionFlagNames` and this branch's merge adds `"format"` to the roster in `TestParseArgsRecordsNoDomainCaptureForASessionFlag`, then re-runs `go test ./cmd/dinah/` on the merged result and confirms it green. If it has not, this card merges as it stands and the obligation falls to dinah-31, whose own AC-10 already carries it.