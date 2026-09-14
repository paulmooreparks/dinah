---
kind: decision
state: resolved
column: 4b38abe7ebd5
owner: holder
ts: 2026-09-14T02:18:38Z
ordinal: 35
note: It re-lays the whole table out. dinah.unresolved-item-exit widens the Refusal column from 19 to 26 columns at an eighty-column window, which narrows the What-can-go-wrong column from 50 to 43 and rewraps four rows that used to fit. cmd/dinah/main_test.go's ratifiedMoveRefusalTable was regenerated from the built binary rather than hand-edited, and it still covers rows 1 to 10 only, which is what it covered before. The alternative was to reuse the profile's unresolved-item name for the exit case, which D-4 already ruled against. Nothing about the rows' order or wording changed, only their column widths.
---
The new refusal name is 26 characters, longer than any name the move's table carried. What does that do to the ratified move refusal table fixture?