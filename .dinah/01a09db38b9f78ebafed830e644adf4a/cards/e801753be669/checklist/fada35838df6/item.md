---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:32Z
ordinal: 8
note: Verified by internal/verb/link_test.go:TestLinkTakesAnyKindAndCanonicalisesNothing, which writes four kinds all naming the same target and asserts all four entries persist with their own kinds, and by TestUnlinkRemovesExactlyOneEntry, which carries two kinds naming one target and removes only the pair named.
---
Two `link` calls on the same card naming the same target under two different kinds both succeed and both entries persist.