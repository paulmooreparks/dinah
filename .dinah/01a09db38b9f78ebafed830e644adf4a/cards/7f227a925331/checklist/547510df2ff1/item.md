---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:18:43Z
ordinal: 14
note: "Verified 2026-09-12 on head 995636e against origin/main at 6fbb3fe: git diff --unified=0 over docs/spec/core-profile.md shows exactly seven removed lines, each above the first changelog entry heading (the four version-identity lines, the section 10 tally, the section 11 index count, and the current-revision line), so no published entry including the 1.0 entry and the 0.12 and 0.13 consequence prose is touched, and zero removed lines carry a published [CORE- statement, which is DOC-VER-4. go test ./internal/profile passed whole on the same head, carrying TestAnEditorialAmendmentMovesNeitherVersionNorChangelog and TestAllOfTheProfilesRevisionStatementsAgree."
---
No published text in the profile is edited. `git diff --unified=0 origin/main -- docs/spec/core-profile.md` shows no removed or modified line at or below the first changelog entry heading, which is `### 1.0` at `:1566` on trunk and not the `### 2.0` at `:1586` that D-8 cites for a different reason, so every published entry including the 1.0 entry and the 0.12 and 0.13 consequence-for-a-caller prose stays byte-identical, and the only addition inside section 12 is the new 0.14 entry appended after the last one. The same diff shows no removed or modified line carrying a published `[CORE-` statement, which is DOC-VER-4. `go test ./internal/profile -run 'TestAnEditorialAmendmentMovesNeitherVersionNorChangelog|TestAllOfTheProfilesRevisionStatementsAgree'` passes.