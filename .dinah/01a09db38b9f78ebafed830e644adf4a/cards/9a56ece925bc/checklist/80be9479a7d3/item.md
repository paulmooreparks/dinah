---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:17Z
ordinal: 7
note: "Verified at 7d50d5b. `grep -rn 'AnchorOf(' --include=*.go .` returns: internal/bench/fields.go:279 (the declaration), internal/bench/entity.go:739 (the one call, inside AnchorPathOf), internal/bench/fields_test.go:143 and :152, and seven lines in cmd/dinah/gate_test.go naming the helper columnAnchorOf, whose name merely ends in the searched text. That helper is new on trunk since the spec was written and is not a join of an anchor filename. The two hand-joins in internal/verb/fields.go are gone; both sites read through AnchorPathOf, at what are now lines 279 and 328 after ab5debc (the spec quotes the pre-ab5debc lines 222 and 271)."
---
The anchor join exists in one place. Running `grep -rn 'AnchorOf(' --include=*.go .` over the whole tree returns only the declaration of `AnchorOf` in `internal/bench/fields.go`, the one call inside `AnchorPathOf`, and the calls in `internal/bench/fields_test.go`; the two hand-joins in `internal/verb/fields.go` at lines 222 and 271 are gone and read through `AnchorPathOf` instead. Paste the command's output into the verification note.