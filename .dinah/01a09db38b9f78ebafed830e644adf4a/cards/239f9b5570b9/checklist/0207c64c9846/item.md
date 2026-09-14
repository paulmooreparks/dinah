---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:51Z
ordinal: 17
note: Verified on the branch head. Two plants, both compiling and both running. Adding `kinds[segment.Kind] = segment.Kind` went red three times at items_test.go:120, naming acceptance_criterion, decision and open_question as accepted while no entry declares them. Setting the acceptance_criterion entry's Word to "ac" went red at items_test.go:102, `acceptance_criterion declares one spelling twice, so nothing distinguishes the word from the short form`. Restored byte-identically after each, cmp clean, green again.
---
The mapping is declared once and both directions are derived from it, so the spelling a reference resolves by and the spelling it is composed from cannot drift apart. internal/bench/resolve.go declares checklistSegments as three entries of kind, word and short form, and builds checklistKinds and itemKindWord from that slice. internal/bench/items_test.go, TestWordForItemKindAgreesWithWhatAReferenceResolvesBy requires every declared word and short form to resolve to its kind, every kind to compose its word, word and short form to differ, and, the arm that catches a second literal, every segment the resolver accepts to come from an entry of checklistSegments. Command: go test ./internal/bench -run TestWordForItemKindAgreesWithWhatAReferenceResolvesBy. Arm it by adding a spelling to checklistKinds that no entry declares.