---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:31Z
ordinal: 3
note: "Verified by internal/verb/link_test.go:TestLinkResolvesATargetInEitherHalfOfTheCollection. The test first fails loudly if the live half still resolves the archived reference, so the archive route is the one exercised, then requires \"to: <archived id>\" in the anchor."
---
Given a target that resolves only in the archived half of the collection, `link` succeeds and stores that card's identifier.