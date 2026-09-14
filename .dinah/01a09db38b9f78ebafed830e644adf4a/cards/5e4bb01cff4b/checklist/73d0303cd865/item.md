---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:17:00Z
ordinal: 8
note: "Test-stage re-verification. All 12 named subtests/tests PASS on the merged tree (verbose run grepped for PASS/FAIL lines). `git diff origin/main...HEAD --stat -- internal/msg/msg_test.go` reports \"118 insertions(+)\", 0 deletions: purely additive."
---
TestEveryDeclaredLanguageShips, TestEveryKeyCarriesAContext, TestPlaceholdersAreNamed, TestMissingKeysFallBackPerKey, TestTheProductNameStaysLatinInEveryLocale, TestRegionalTagsWalkTheHierarchy, TestPluralsFollowTheCategories, TestATranslationKeepsThePlaceholdersAndTheSplice, TestATranslationTracksItsEnglishSource and TestEveryUntranslatableIdentifierSurvivesTranslation all continue to pass unmodified; none is deleted, weakened, or subsumed by a new guard.