---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:03Z
ordinal: 4
note: "Both directions planted and observed at c25c20f. Removing `archive` from the workstream sentence failed with `references_guide_test.go:223: `dinah archive workstream/<slug>` works and the references guide's workstream sentence does not name archive`. Restoring it and adding `show` failed with `references_guide_test.go:228: the references guide's workstream sentence names show and `dinah show workstream/<slug>` does not take one`. Restored byte-identically, test ok. The six names were re-derived at this commit by probing all fifteen commands against a fresh workstream each: path, edit (exit 4, unreachable exec, so the address resolved), archive, delete, contents and attachments take one; the other nine refuse."
---
Remove the token `` `archive` `` from the guide's sentence beginning `Six commands take a workstream:` and run `go test ./cmd/dinah/ -run TestTheReferencesGuideNamesTheCommandsThatTakeAWorkstream`. It fails with ``dinah archive workstream/<slug>` works and the references guide's workstream sentence does not name archive`. Then restore it and add `` `show` `` to the sentence instead: it fails with `the references guide's workstream sentence names show and `dinah show workstream/<slug>` does not take one`.