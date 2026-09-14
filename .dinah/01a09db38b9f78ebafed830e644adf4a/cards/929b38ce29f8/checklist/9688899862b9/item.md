---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:52Z
ordinal: 3
note: "`go test ./internal/profile/... -run TestAnEditorialAmendmentMovesNeitherVersionNorChangelog` passes. `git show 50c7442 --stat` names only docs/spec/core-profile.md and internal/profile/extract_test.go, so amendment_test.go's three constants are untouched by this card."
---
go test ./internal/profile/... passes with the three constants in internal/profile/amendment_test.go unchanged from before this card: publishedStatements, declaredVersion (holding section 2's version sentence in full), and publishedChangelogEntries.