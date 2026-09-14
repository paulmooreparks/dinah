---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:16:35Z
ordinal: 9
note: go test ./cmd/dinah/ -run TestParseArgsRecordsNoDomainCaptureForASessionFlag passes for workbench/json/quiet/lang/actor (plus help/version/format/domain-flag-control).
---
A unit test in cmd/dinah/args_test.go asserts that parseArgs never records a domainCapture for any name in sessionFlagNames: run once for each of workbench, json, quiet, lang and actor with a minimal argv that gives the flag a value or marks it present (e.g. {"--lang", "de"}, {"--json"}), asserting len(parsed.domainCaptures) == 0 after each call. This is the direct guard for the invariant "Why the two callers now agree everywhere" names: were a future change to fold a session flag into domainCaptures, this test fails immediately, rather than leaving resolveOpenTailFlags free to rewrite that flag's value on some open-tail command with nothing on record to catch it.