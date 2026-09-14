---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:06Z
ordinal: 4
note: "Verified at 4a8c66a. Test: internal/bench/check_test.go, TestAKindGivenAnAttachmentsMountStopsBeingReported, which gives containment[KindItem] an attachments mount and restores the table in t.Cleanup. Command: go test ./internal/bench/ -run TestAKindGivenAnAttachmentsMountStopsBeingReported. Green. Plant, chosen so this criterion reddens alone: the walk's question rewritten as a list of kinds, `if kind == KindItem || kind == KindAttachment || kind == KindWorkstream`. Red at check_test.go:2376, \"...\\checklist\\d00000000001\\attachments is still reported, and the table now gives an item an attachments mount\", while AC-3's test passed in the same run. The spec's own suggested arming, the two-kind list, reddens both tests at once because it also drops the workstream; that variant was run first and is recorded under AC-3. Restored byte-identically and green again."
---
AC-4. `internal/bench/check_test.go`, `TestAKindGivenAnAttachmentsMountStopsBeingReported`: with the same fixture as AC-3, the test adds an `attachments` mount to `containment[KindItem]`, restores the table in `t.Cleanup`, and asserts that `Bench.Check` then returns no `check.attachments-without-a-mount` finding naming the item's directory while still returning the ones naming the attachment's and the workstream's. This is what proves the walk reads the table rather than a list of kinds.