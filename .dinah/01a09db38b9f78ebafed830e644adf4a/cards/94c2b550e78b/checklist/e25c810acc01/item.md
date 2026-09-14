---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:31Z
ordinal: 6
note: "Verified at 5a88bec. `grep -rn \"referencesGuideProseParagraphs\\|TestTheReferencesGuideDeniesNoCommandTheToolHas\" <worktree> --include=*.go --include=*.md` returns no hits at all, so both the retired reader and the retired check are gone with no caller left behind. `grep -rn \"guideDenialOfACapability\" <worktree> --include=*.go` returns three lines, all in cmd/dinah/guide_guard_test.go: the doc comment at :1068, the single `var guideDenialOfACapability = regexp.MustCompile(...)` declaration at :1076, and its one use at :1154. Exactly one declaration, and the pattern moved unedited rather than being copied."
---
The duplicate prose reader is gone: a tree-wide grep finds no definition or call of referencesGuideProseParagraphs, no definition or call of TestTheReferencesGuideDeniesNoCommandTheToolHas, and exactly one declaration of guideDenialOfACapability. The grep output is recorded.