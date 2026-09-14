---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:17Z
ordinal: 9
note: Verified at 7d50d5b, read after origin/main was fetched and found already merged into the branch (trunk is at ab5debc and the branch carries it). `git diff --stat origin/main -- internal/guide internal/msg` printed nothing. The four named guards all ran unedited in the full cmd/dinah and internal/verb suites, which passed.
---
The guide and the eight message catalogues are untouched. `git diff --stat origin/main -- internal/guide internal/msg` prints nothing on the card's branch, and the whole suite passes with `TestTheReferencesGuideNamesTheCommandsThatTakeAWorkstream`, `TestEveryAddressTheContentsTreeDrawsResolvesThroughTheCommandsThatDeclareIt`, `TestTheReferencesGuideSaysWhichCommandTakesWhat` and `TestATranslationTracksItsEnglishSource` all green and all unedited.