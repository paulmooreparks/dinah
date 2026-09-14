---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:31Z
ordinal: 7
note: "Verified by internal/verb/link_test.go:TestLinkIsIdempotentOnAPairTheCardAlreadyCarries, which repeats the call naming the target by identifier the second time (so idempotency is decided on the resolved pair, not the spelling), then asserts one entry, a byte-identical anchor and exactly one linked event. Armed by deleting the indexOfLink guard: red with 'got [{Kind:duplicates To:c4e7...} {Kind:duplicates To:c4e7...}]'. Restored, green. Also seen through the binary: two identical link calls leave one entry and one linked line in journal.ndjson."
---
A second `link` call with the same source card, kind and resolved target as one already on the card succeeds, writes no duplicate entry, and appends no second journal event.