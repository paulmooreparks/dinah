---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:32Z
ordinal: 10
note: "Verified by internal/verb/link_test.go:TestUnlinkRemovesExactlyOneEntry, which builds three links (two of them naming one target), removes the middle one, and asserts the survivors element by element in order. Armed by replacing the matched-index removal with reloaded.Links[1:]: red with 'entry 0 is {Kind:relates To:584c...}, wanted {Kind:duplicates To:47b5...}, so removal did not keep the order'. Restored, green."
---
`unlink <card> <kind> <to>` removes exactly the one entry whose kind and resolved target match, leaving every other entry on the card in its original order.