---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:51Z
ordinal: 16
note: "Verified on the branch head. Plant: ItemView.Kind filled from WordForItemKind, gofmt clean, `go build ./...` exit 0 and the run executed. Red three times, once per token: `the payload carries no \"open_question\", and the kind tokens do not change with the addressing`, and the same for \"acceptance_criterion\" and \"decision\". Restored, cmp clean, green again."
---
The kind tokens on the machine surface are unchanged by the addressing rename. In the same test, `dinah show fx-1 --json` must carry `"open_question"`, `"acceptance_criterion"` and `"decision"` after the rename has landed. Command: go test ./cmd/dinah -run TestAChecklistItemIsAddressedByAWordAndTheShortFormStillResolves. Arm it by filling ItemView.Kind in internal/verb/read.go from bench.WordForItemKind(item.Kind) instead of item.Kind, which is the exact confusion of the two questions this criterion guards against, and which compiles.