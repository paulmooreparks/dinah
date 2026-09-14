---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:17:11Z
ordinal: 6
note: "Command run from the repository root of the worktree (C:/dinah-scratch/dinah-312-impl/wt): `grep -rn \"RecognizedAt(\" --include=*.go .` Output: (empty), exit status 1. Before the change the same command returned two lines, the definition at internal/bench/vocabulary.go:137 and the call at internal/verb/vocabulary.go:88. Both are gone: the function is deleted, not merely uncalled, and `go build ./...` and `go vet ./...` are clean, so nothing references it."
---
grep -rn "RecognizedAt(" --include=*.go . run from the repository root returns no matches, proving the retired function is fully removed rather than merely unused.