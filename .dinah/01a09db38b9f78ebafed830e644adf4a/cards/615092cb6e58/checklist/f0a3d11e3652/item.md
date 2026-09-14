---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:15Z
ordinal: 21
note: "dinah-456 section 9 and this card's own description say card 6 wants card 5's journalling in place. Nothing in this contract reads or writes anything dinah-460 lands: restore journals one restored event through bench.AppendEvent, which Library.Archive already uses, and it touches no field write, no FieldsOf and neither generic verb. So the ordering is a preference and either card may land first. The real overlap is verb.params, verb.guides, cmd/dinah's command table, internal/mcp's tools slice, references_guide_test.go's referenceProbeArgs, and the references guide's command table. Every one is a list and neither card's entries exclude the other's, so whichever lands second keeps both sets and re-runs TestTheReferencesGuideTableNamesEveryCommandThatTakesAReference, which fails on a missing row and on a surplus one. dinah-460 also reshapes toolExemptions into a struct of two fields, and this card does not touch that map."
---
This card does not depend on dinah-460, and the two cards collide only in five lists whose merge is caught by an existing derivation test.