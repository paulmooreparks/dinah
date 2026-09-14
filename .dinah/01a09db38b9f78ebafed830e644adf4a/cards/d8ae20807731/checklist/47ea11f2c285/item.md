---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:40Z
ordinal: 11
note: fx-3 is the archive-only collision. ResolveLinkTarget("fx-3") and watchedCard("fx-3") each answer an empty identifier and dinah.ambiguous-card carrying both archived identifiers; changes --card fx-3 exits non-zero on the head; changes --card 0f8d904a8b4c, which no card file backs, still exits 0 through the unchanged third arm.
---
An ambiguity in the archived half is refused on its own terms by both callers that read that half, rather than being flattened to unknown-card. Against the bench-package fixture in the spec, whose live half holds no card on number 3 and whose archive holds c00000000005 and c00000000006 both on number 3, a direct call to ResolveLinkTarget("fx-3") and a direct call to Library.watchedCard("fx-3") each answer an empty identifier and a refusal named dinah.ambiguous-card, and each refusal's Extra carries both identifiers. The CLI leg drives the same path against the CLI fixture built the same way: `runCLI(t, root, "changes", "--card", "fx-3")` exits non-zero and its stderr carries dinah.ambiguous-card. A second leg holds the third arm of watchedCard open: `runCLI(t, root, "changes", "--card", <a well-formed 12-hex identifier no card file backs>)` still exits 0, because an identifier is never ambiguous and that arm is unchanged.