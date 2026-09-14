---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:18:43Z
ordinal: 11
note: "Verified 2026-09-12 on head 995636e: go test ./internal/profile passed whole in 46.3s with CORE-CARD-10 published, holding the extraction, the section 4 vocabulary entry, the section 10 boundary row and its tally (36/58 to 37/59), the section 11 index row and its tally (138 to 139), the appended 0.14 changelog entry, and the five present-tense revision sites against one another. TestAllOfTheProfilesRevisionStatementsAgree is the one that fails if fewer than five move, and it passed. internal/profile/amendment_test.go carries declaredVersion and declaredCurrentRevision at 0.14, publishedStatements 139, publishedChangelogEntries 14."
---
The profile amendment publishes 0.14, is internally consistent, and edits nothing already published. `go test ./internal/profile` passes with CORE-CARD-10 published, which holds the extraction, the section 4 vocabulary entry, the section 10 boundary row and its tally (36 to 37, 58 to 59), the section 11 index row and its tally (138 to 139), the new changelog entry, and the five present-tense revision sites against one another; `TestAllOfTheProfilesRevisionStatementsAgree` is the one that fails if fewer than five move. `internal/profile/amendment_test.go` carries declaredVersion and declaredCurrentRevision at 0.14, publishedStatements 139 and publishedChangelogEntries 14. The revision is established by reading `docs/spec/core-profile.md:3`, `:1559` and the last entry heading rather than taken from this spec.