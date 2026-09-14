---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:38Z
ordinal: 2
note: "Drives both halves: the guard passes on the corrected document, and it is armed against each of the two constants it derives from. The -v is there so a vacuous pass, which this workbench has shipped three times, shows up as no test having run."
---
`go test ./cmd/dinah/ -run TestTheQuickStartWorkbenchFileDeclaresWhatTheBinaryWrites -v` passes and reports at least one test run, and the same command reports a failure naming docs/quick-start.md when `ProfileMinor` in internal/bench/bench.go is set to 11 and again when `StorageFormat` is set to 1.