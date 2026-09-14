---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:40Z
ordinal: 5
note: TestALinkTargetRefusesAnAmbiguousNumber (internal/bench) answers an empty identifier and dinah.ambiguous-card for fx-1; TestALinkRefusesAnAmbiguousCardNumber (cmd/dinah) drives link fx-2 relates-to fx-1 non-zero and reads the subject's anchor back to confirm no link was written.
---
A link naming a live-half ambiguous number is refused rather than falling through to the archive. Against the bench-package fixture in the spec, whose live half holds c00000000001 and c00000000002 both on number 1 plus c00000000003 on number 2, and whose archive holds c00000000004 on number 1, a direct call to ResolveLinkTarget("fx-1") answers an empty identifier and a refusal named dinah.ambiguous-card rather than the archived card's identifier. The same fixture drives the end-to-end leg: `runCLI(t, root, "link", "fx-2", "relates-to", "fx-1")` exits non-zero, its stderr carries dinah.ambiguous-card, and c00000000003's anchor records no link afterwards.