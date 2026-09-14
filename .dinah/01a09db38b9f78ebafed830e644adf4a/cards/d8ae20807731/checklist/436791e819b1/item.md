---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:40Z
ordinal: 2
note: TestAnAmbiguousCardNumberIsRefusedOnTheHead drives show fx-1, show 1 and path fx-1 against the CLI fixture; all three exit non-zero with dinah.ambiguous-card on stderr.
---
A number two cards carry is refused on the CLI head. Against the CLI fixture in the spec, whose two live cards both carry number 1, `runCLI(t, root, "show", "fx-1")` exits non-zero and its stderr carries `dinah.ambiguous-card`, and the same holds for the bare number form `runCLI(t, root, "show", "1")` and for `runCLI(t, root, "path", "fx-1")`.