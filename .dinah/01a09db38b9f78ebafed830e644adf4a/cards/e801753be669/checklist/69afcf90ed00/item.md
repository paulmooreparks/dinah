---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:32Z
ordinal: 11
note: "Verified by internal/verb/link_test.go:TestUnlinkRemovesExactlyOneEntry's second half, which names a real kind and a real target that no entry pairs, asserts contract.UnknownLink, asserts the detail is the target as typed and the context carries the kind, and compares the anchor before and after. Also seen through the binary: `dinah unlink ac21-1 relates ac21-2` exits 2 with \"dinah.unknown-link this card carries no relates link to ac21-2\"."
---
`unlink` naming a (kind, to) pair the card does not carry refuses `dinah.unknown-link` and writes nothing.