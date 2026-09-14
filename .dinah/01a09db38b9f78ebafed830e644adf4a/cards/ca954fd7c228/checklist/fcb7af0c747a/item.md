---
kind: decision
state: resolved
owner: operator
ts: 2026-09-14T02:17:25Z
ordinal: 27
note: "Agent design review found `TestATranslationKeepsThePlaceholdersAndTheSplice` iterating `Complete`, which is `en`, `hi`, `de`, so the five skeleton catalogues go unread by the guard that checks the direction this card was already extending. Confirmed by reading msg.go: `Complete = []string{Base, \"hi\", \"de\"}`, `Skeleton` holds the other five, and `Tags()` returns all eight. That is the same defect this card exists to close, one layer down, in a file the card already opens. Reviewer called it a two-line fix and left the call to the operator because it changes a guard the card was not asked to touch. Ruled by Claude Opus 5 under standing fast-track authority: it rides along. The reason the extension keeps its five skeletons in the placeholder walk (D-6) applies here unchanged, so the CLI's two halves would otherwise disagree with each other and with the extension."
---
The existing CLI placeholder test is widened to all eight catalogues on this card rather than in a follow-up.