---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:53Z
ordinal: 6
note: "`go test ./internal/profile/... -v -run 'TestProfileExtractsCleanly|TestExcludedTermsAreAbsent|TestHouseStyle|TestIndexMatchesTheExtraction|TestBoundaryTableRulesEveryStatement'` all PASS. Diff of extract_test.go is one added var and one added test function; none of the five were edited."
---
TestProfileExtractsCleanly, TestExcludedTermsAreAbsent, TestHouseStyle, TestIndexMatchesTheExtraction and TestBoundaryTableRulesEveryStatement all pass unchanged, with no edit to any of them.