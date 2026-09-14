---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:17:33Z
ordinal: 22
note: "format.md's own \"Card-to-card links\" prose settles the open vocabulary independently (\"The kind is an open enum... A workbench writing something else is conforming\"), under format.md's own \"Decisions recorded here are settled unless reopened\" rule, and the operator's 2026-09-09 ruling that Dinah must stay usable for a workbench with no code, no merge and no tests backs it further. core-profile.md states the same rule as CORE-LINK-4, but that revision (`dinah-core 0.12`, maturity channel `dev`) explicitly says \"nothing below binds\" at its current channel, and `internal/profile/conformance_test.go` still lists CORE-LINK-4 as `outOfReach` for that reason. CORE-LINK-4 is cited here as drafted and convergent with format.md's already-settled answer, not as the source of the obligation. Not an operator call: the operator's own ruling is already what grounds it."
---
D-1: link kind stays a fully open string, never a closed or Andoneer-mapped vocabulary.