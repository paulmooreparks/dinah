---
kind: decision
state: resolved
column: c9428b3bc921
ts: 2026-09-14T02:17:13Z
ordinal: 9
note: tree.go already threads the resolved column into States() and already has a carried-value mechanism (axisValueOrder) that draws a group for any value a card actually carries beyond what a closed axis declares, built for exactly this shape of case by dinah-275 AC-4. Fixing the one place the rule is declared lets every existing reader answer correctly with no new parameter and no second copy of the occupancy rule, which is what the single-predicate discipline this workstream (dinah-207, dinah-253, dinah-273, dinah-275) asks for.
---
The fix lives in internal/bench.Column.States() alone; internal/verb/tree.go's grouping recursion needs no code change, only doc-comment corrections.