---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:02Z
ordinal: 1
note: "Planted and observed at c25c20f in C:/dinah-scratch/dinah-457-impl/wt. Deleted the `| reopen | ... |` row from internal/guide/guides/references.md and ran `go test ./cmd/dinah/ -run TestTheReferencesGuideTableNamesEveryCommandThatTakesAReference`. It compiled, ran, and failed with both messages verbatim: `references_guide_test.go:126: reopen points at the references guide and the guide's table carries no row for it` and `references_guide_test.go:136: the table draws 14 rows against a roster of 15: [archive attach ... verify]`. Restored the row from a byte-identical copy, `git status --short` empty, test ok."
---
In a worktree carrying the change, delete the `| reopen | ... |` row from the table in `internal/guide/guides/references.md` and run `go test ./cmd/dinah/ -run TestTheReferencesGuideTableNamesEveryCommandThatTakesAReference`. It fails with `reopen points at the references guide and the guide's table carries no row for it`, and with `the table draws 14 rows against a roster of 15`. Restore the row and it passes.