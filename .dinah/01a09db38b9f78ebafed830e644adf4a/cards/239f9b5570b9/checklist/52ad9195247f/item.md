---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:51Z
ordinal: 15
note: "Verified on the branch head. Plant: that one line added. `go build ./...` exit 0 and the run executed. Red three times at address_sweep_test.go:1008, naming `fx-1/decision/1`, `fx-1/open_question/1` and `fx-1/acceptance_criterion/1` as accepted with the item.md each opened, and red three times in internal/bench at items_test.go:120 for the undeclared segments. `fx-1/question/1` and `fx-1/criterion/1` stayed refused under the plant, correctly, since the plant taught the resolver kind tokens rather than singulars. Restored, cmp clean, green again."
---
A spelling nothing declares is refused, so the rename cannot be satisfied by a resolver that accepts everything. In the same test, `dinah path` is handed `fx-1/question/1`, `fx-1/criterion/1`, `fx-1/decision/1`, `fx-1/open_question/1` and `fx-1/acceptance_criterion/1`, and every one must exit non-zero. The last three are the sharp ones: they are real kind tokens, and this ruling is about addressing rather than about kinds. Command: go test ./cmd/dinah -run TestAChecklistItemIsAddressedByAWordAndTheShortFormStillResolves. Arm it by adding `kinds[segment.Kind] = segment.Kind` to checklistKinds in internal/bench/resolve.go, which compiles and turns the refusal assertion red.