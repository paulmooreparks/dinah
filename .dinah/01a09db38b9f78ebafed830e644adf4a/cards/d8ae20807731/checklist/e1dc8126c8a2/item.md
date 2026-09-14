---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:40Z
ordinal: 9
note: TestTheChangesCardFilterRefusesAnAmbiguousNumber (internal/verb) drives watchedCard("fx-1") to an empty identifier and dinah.ambiguous-card; TestTheChangesFilterRefusesAnAmbiguousCardNumberOnTheHead (cmd/dinah) drives changes --card fx-1 non-zero with the archived identifier absent from the output, and changes --card of the archived card's own identifier still exits 0.
---
The `changes` card filter refuses an ambiguous number rather than watching the archived card. Against the bench-package fixture in the spec, whose live half holds two cards on number 1 and whose archive holds one, a direct call to `Library.watchedCard("fx-1")` answers an empty identifier and a refusal named `dinah.ambiguous-card`. The CLI leg drives the same path against the CLI fixture built with an archived card on number 1: `runCLI(t, root, "changes", "--card", "fx-1")` exits non-zero and its stderr carries `dinah.ambiguous-card`, and the archived card's identifier appears nowhere in the output. A second leg proves the refusal is not a blanket one: `runCLI(t, root, "changes", "--card", <the archived card's 12-hex identifier>)` still exits 0, because an identifier is never ambiguous and the third arm of `watchedCard` is unchanged.