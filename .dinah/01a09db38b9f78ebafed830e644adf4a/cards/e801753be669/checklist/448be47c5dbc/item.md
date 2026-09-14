---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:32Z
ordinal: 17
note: "`go test ./internal/profile` passes. `grep -n \"CORE-LINK\" internal/profile/conformance_test.go` over the whole file returns nothing, so no outOfReach entry survives for any of the six. `go test ./internal/profile -run TestConformanceReport -v` shows each statement mapped to a driving test: CORE-LINK-1 TestLinkWritesOneEntryToTheCardsOwnAnchor; CORE-LINK-2 that test plus TestLinkResolvesATargetInEitherHalfOfTheCollection; CORE-LINK-3 TestLinkRefusesATargetTheWorkbenchDoesNotCarry; CORE-LINK-4 TestLinkTakesAnyKindAndCanonicalisesNothing; CORE-LINK-5 TestNoVerbRefusesOnAccountOfALink; CORE-LINK-6 TestLinkWritesNothingToTheCardItNames. The tests live in internal/verb because that is where the behaviour is and where every other CORE statement's driver lives (drivenStatements walks the whole tree, and outOfReachDefects fails a row a test now drives, so the profile package is what enforces the mapping)."
---
`go test ./internal/profile` passes with CORE-LINK-1 through CORE-LINK-6 removed from `outOfReach` and each exercised by a passing test in that package; `grep -n "CORE-LINK" internal/profile/conformance_test.go`, run over the whole file, shows no `outOfReach` entry for any of the six.