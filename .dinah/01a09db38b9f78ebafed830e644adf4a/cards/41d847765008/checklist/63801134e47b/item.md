---
kind: decision
state: resolved
column: c9428b3bc921
ts: 2026-09-14T02:17:13Z
ordinal: 12
note: "It validates whether a literal filter value is a member of the state vocabulary at all, independent of any column, and never draws a group or claims a card can reach the value; same ground as dinah-275's D-3 for the identical function. A card at an intake column can, after a hand edit, genuinely carry state: active (dinah check reports the anomaly via FindingClaimWhereNoWorkIsTaken but does not refuse it, per format.md's \"Manual edits are witnessed, not prevented\"), so narrowing this validation by column would be a new and unrelated restriction."
---
internal/verb/query.go's closedValues(FieldState), the full ready/active/blocked triple validating a typed filter term such as state:active, stays untouched.