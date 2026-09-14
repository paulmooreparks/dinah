---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:40Z
ordinal: 12
note: "TestTheEditTargetRefusesAnAmbiguousNumber (internal/bench) and TestEditRefusesAnAmbiguousCardNumber (cmd/dinah): ResolveEditTarget answers an empty path and dinah.ambiguous-card, because the retry through ResolvePath descends to the same resolveCardIn and raises it again."
---
The one discard-and-retry site this card leaves alone is driven rather than argued. Against the CLI fixture in the spec, `runCLI(t, root, "edit", "fx-1")` exits non-zero and its stderr carries dinah.ambiguous-card, and a Go test in internal/bench asserts the same directly: ResolveEditTarget("fx-1") against a bench built on the bench-package fixture answers an empty path and a refusal named dinah.ambiguous-card. The edit leg needs no editor, because the refusal is raised during reference resolution and before any editor is launched.