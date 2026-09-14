---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:31Z
ordinal: 1
note: "Verified by internal/verb/link_test.go:TestLinkWritesOneEntryToTheCardsOwnAnchor, whose assertion reads the raw card.md and requires the exact block \"links:\\n  - kind: duplicates\\n    to: <id>\". Armed: with ResolveLinkTarget returning the caller's spelling the guard went red naming the wrong value (\"the anchor of fx-1 carries no links block of the shape ... got ... to: fx-2\"); restored from the commit and green again."
---
`dinah link <card> <kind> <to>` writes one `{kind, to}` entry to the `links:` sequence in the source card's own card.md frontmatter, verified by reading the raw frontmatter after the call.