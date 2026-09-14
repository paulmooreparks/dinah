---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:38Z
ordinal: 31
note: "Yes, and the sweep that finds every affected site must run over the whole tree rather than over the files the placement fix already touched. A widened re-sweep (grep for row-number and count-word patterns across every .go/.md/.json file, every hit read in context) found thirteen sites needing correction: twelve Go-source sites across internal/verb/checks.go (three), internal/verb/pull.go (three), and internal/verb/pull_test.go (five), plus a thirteenth in cmd/dinah/main_test.go, whose subtest name \"the thirteen checks in order\" is folded into this same renumbering rather than left recorded as a deliberate omission (Agent Design Review's third-pass finding). That thirteenth site's own number was already wrong before this card for an unrelated reason (the true count on trunk today is seventeen, not thirteen); the fold-in lands it on the number that becomes true once this card ships: eighteen, verified as WorkbenchChecks' two entries (unchanged) plus pullChecks' sixteen rows after this card's insertion. The full enumeration, with exact before/after text for each site, is in the spec's \"The re-swept enumeration\" section, which now carries all thirteen and no longer records a deliberate omission in Out of Scope."
---
Pull reuses canLand, so does the exit-hold check's corrected position require renumbering pullChecks, and if so how much?