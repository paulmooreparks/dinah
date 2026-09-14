---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:32Z
ordinal: 15
note: "Two halves. `git diff origin/main --stat -- internal/bench/check.go` returns nothing, so the dangling-link logic is untouched by this card's diff. And the finding still fires: internal/verb/beyond_test.go:TestArchiveAndDelete deletes a card another card's link names and requires bench.FindingDanglingLink in the report; it passes. That test's fixture was the one behavioural change, from planting the raw frontmatter block to setting Card.Links, because Save now renders the block from the typed field the way it already did for tier_at."
---
`dinah check`'s existing `check.dangling-link` finding continues to fire, unchanged, for a link whose target was deleted after the link was written; `internal/bench/check.go`'s dangling-link logic is untouched by this card's diff.