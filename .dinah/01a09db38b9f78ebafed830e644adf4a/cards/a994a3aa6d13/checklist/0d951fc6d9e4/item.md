---
kind: decision
state: resolved
ts: 2026-09-14T02:16:51Z
ordinal: 38
note: "Resolved at start of implement (Agent Design Review cycle 3 returned it to pending under the column's rule that a decision justified by a false claim is one the operator never saw the real tradeoff on). The shipped sentence \"the workbench you name lies under the root\" stays at 42 columns and is safe. The explanation is rewritten at implement: the real ceiling is 52 display columns (refusal name stays beside the sentence); the name moves off the row at 53; the widest table line stays at or under 80 until the check sentence itself overruns, which first happens at 72. AC-24's arming break goes to column 53, not 51. Spec catalog section, body note \"Why the check sentence is short rather than declared\", AC-24's note, and the UX sketch sections 5 and 7 are redrawn to match."
---
check.mcp.2 is shortened to fit its column rather than left long with the two-line layout declared.