---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:03Z
ordinal: 5
note: "Both arms planted and observed at c25c20f. Appending `<!-- DinahPath -->` to internal/guide/guides/query.md failed with `references_guide_test.go:369: internal/guide/guides/query.md:148 carries the name DinahPath and testdata\\dinahpath-allowlist.txt does not name that file`, naming the file and the line. Restored, then rewrote the guide's opening sentence to say \"the path language\" instead: it failed with `references_guide_test.go:381: internal/guide/guides/references.md carries no occurrence of the name DinahPath, and a guard satisfied by deleting the name is no guard`. Restored byte-identically, test ok."
---
Write the word DinahPath into a comment in `internal/guide/guides/query.md` and run `go test ./cmd/dinah/ -run TestTheNameDinahPathStandsOnlyWhereItIsDeclared`. It fails naming `internal/guide/guides/query.md` and the line the word stands on. Remove it, and instead delete the word from `internal/guide/guides/references.md`: it fails saying the references guide carries no occurrence of the name and that a guard satisfied by deletion is no guard.