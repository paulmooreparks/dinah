---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:51Z
ordinal: 14
note: "Verified on the branch head. Plant: that one line removed. `go build ./...` exit 0 and the run executed. Red at address_sweep_test.go:996, `[path fx-1/oq/1]: 2 dinah.unknown-path nothing in this workbench answers to oq`, and red in internal/bench at items_test.go:106 for all three short forms. Restored, cmp clean, green again."
---
The three short forms go on resolving on input and open exactly what the word opens. In the same test, each of `fx-1/oq/1`, `fx-1/ac/1` and `fx-1/d/1` is handed to `dinah path` alongside its word, and the two answers must be the same file; the comparison is between two answers from the tool rather than against a path the test builds, so it cannot agree with itself. Command: go test ./cmd/dinah -run TestAChecklistItemIsAddressedByAWordAndTheShortFormStillResolves. Arm it by deleting the `kinds[segment.Short] = segment.Kind` line from checklistKinds in internal/bench/resolve.go, which compiles and turns the parity assertion red.