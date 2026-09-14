---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:32Z
ordinal: 12
note: Verified by internal/verb/link_test.go:TestUnlinkRemovesExactlyOneEntry, which writes the link by human reference and removes it by the bare 12-hex identifier. Both verbs share one helper, Library.linkArguments, so the resolution cannot diverge; the test proves the shared path answers the same pair from either spelling.
---
`unlink`'s `to` argument resolves by the identical rule `link`'s does, so a caller may remove a link using either the reference they created it with or the bare identifier.