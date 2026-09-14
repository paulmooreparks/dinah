---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:32Z
ordinal: 9
note: "Verified at 5a88bec. internal/guide/guidepin/guidepin.go declares Statement, Pinned and Carries, and imports only errors, fmt, strings and dinah/internal/guide, so it imports neither testing nor internal/bench (the reviewer's nit applied: that clause is true of guidepin.go, and its own test file does import testing, as any test file must). `grep -rln \"internal/guide/guidepin\" <worktree> --include=*.go` returns exactly three files, all of them _test.go: cmd/dinah/guide_pin_test.go, cmd/dinah/restore_test.go and internal/bench/resolve_archived_half_test.go. Filtering that list to non-test files returns nothing, so no production file imports the package. Modelled on internal/bench/compattest, which exists on the same terms."
---
internal/guide/guidepin exists, declares Statement, Pinned and Carries, imports neither testing nor internal/bench, and no non-test file in the tree imports it. Verified by grep for the import path across non-test Go files, with the output recorded.