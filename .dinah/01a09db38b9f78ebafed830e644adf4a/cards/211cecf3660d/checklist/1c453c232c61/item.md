---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:27Z
ordinal: 8
note: "Re-verified via full suite: go test ./... -count=1 passed, including go test ./internal/msg/ (TestEveryKeyCarriesAContext, TestATranslationTracksItsEnglishSource for all 7 non-English locales, etc, all PASS). Full sweep run because cmd/ is touched by this diff."
---
The catalog key check.item-column-unresolved exists in all eight locale files under internal/msg/locales/, and the message-catalog completeness tests in internal/msg/msg_test.go (including TestEveryKeyCarriesAContext) pass with it added.