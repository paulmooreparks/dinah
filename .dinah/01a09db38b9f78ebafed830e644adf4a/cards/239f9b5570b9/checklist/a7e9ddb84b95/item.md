---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:51Z
ordinal: 13
note: "Verified on the branch head. Plant: internal/bench/resolve.go, the acceptance_criterion entry changed to {Kind: \"acceptance_criterion\", Word: \"ac\", Short: \"ac\"}. `go build ./...` exit 0 and the run executed. Red at address_sweep_test.go:982, `show still prints the short form \"fx-1/ac/1\", and a word is what it composes now`, with the failure printing the screen showing `fx-1/questions/1` and `fx-1/decisions/1` beside `fx-1/ac/1`, and red again at :995 because `path fx-1/criteria/1` then refused with dinah.unknown-path. Restored from a copy taken before the plant, cmp clean, green again."
---
`dinah show <card>` draws a checklist item's reference with the kind spelled as a word, and that word resolves. In cmd/dinah/address_sweep_test.go, TestAChecklistItemIsAddressedByAWordAndTheShortFormStillResolves files one item of each kind on fx-1 and requires the rendered screen to carry `fx-1/questions/1`, `fx-1/criteria/1` and `fx-1/decisions/1`, and to carry none of `fx-1/oq/1`, `fx-1/ac/1`, `fx-1/d/1`. Command: go test ./cmd/dinah -run TestAChecklistItemIsAddressedByAWordAndTheShortFormStillResolves. Arm it by editing the acceptance_criterion entry of checklistSegments in internal/bench/resolve.go to Word: "ac", which compiles and turns the printed-form assertion red.