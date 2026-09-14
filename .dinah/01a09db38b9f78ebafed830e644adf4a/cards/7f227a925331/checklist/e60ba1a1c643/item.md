---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:18:42Z
ordinal: 4
note: "Verified 2026-09-12 on head 995636e: go test ./cmd/dinah -run TestTwoBranchesFilingACardConflictOnTheRegistry passed in 1.89s, built on the cmd/dinah/container_test.go precedent the criterion names. The test drives two branches each filing a card through the CLI from one committed base, asserts git merge exits non-zero with card-numbers.txt in conflict, and that the .gitattributes a new workbench is written with names no pattern covering the registry (also held by TestTheRegistryIsNotUnionMerged)."
---
Two clones filing a card from one base produce a git conflict. `go test ./cmd/dinah -run TestTwoBranchesFilingACardConflictOnTheRegistry` passes, built on the precedent at cmd/dinah/container_test.go:285: it inits a workbench, commits it, files a card on each of two branches through the CLI, merges, and asserts that `git merge` exits non-zero with `card-numbers.txt` in conflict. The same test asserts that `.gitattributes` names no pattern covering the registry.