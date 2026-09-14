---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:19Z
ordinal: 6
note: Verified. `go test ./internal/msg/ -run TestTheReferenceKindKeysReachEveryCatalogue -v -count=1` passes and logs "8 catalogue files enumerated, 56 entries read" (internal/msg/reference_kind_keys_test.go, the t.Logf and the t.Fatalf count guard after it). Modeled line for line on internal/msg/collection_keys_test.go:TestTheCollectionKeysReachEveryCatalogue. Both plants armed. Deleting hi's reference.kind.card reddened "hi carries no entry for reference.kind.card" plus the count guard at 55. Setting de's reference.kind.column to the English text reddened "de carries the English text for reference.kind.column under its own tag" with the count still 56, which is the failure a fingerprint alone cannot see and why the test asserts difference rather than presence. Restored byte-identically after each and green.
---
`go test ./internal/msg/ -run TestTheReferenceKindKeysReachEveryCatalogue -v` passes and logs eight catalogue files and fifty-six entries read; deleting `hi`'s `reference.kind.card` reddens naming hi and that key, and setting `de`'s `reference.kind.column` to the English text reddens naming de for carrying the English under its own tag.