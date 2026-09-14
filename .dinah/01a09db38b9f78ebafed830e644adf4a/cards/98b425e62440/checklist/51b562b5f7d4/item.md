---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:16:34Z
ordinal: 2
note: go test ./cmd/dinah/ -run TestARefusalReachesItsReaderInTheirOwnLanguage passes on merged tree.
---
TestARefusalReachesItsReaderInTheirOwnLanguage (main_test.go:4566), asserting dinah --lang hi add --nosuchflag Thing renders in Hindi, passes unchanged after the fix.