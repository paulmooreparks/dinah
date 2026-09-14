---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:48Z
ordinal: 18
note: "Compares the spawn log and the fake host's call log against expected values. The second clause is the accepting partner: a `rowOnly` implementation that refuses everything would satisfy the first clause on its own, and the workbench's rule is that a criterion asserting something is refused must pin the accepting case beside it."
---
New Card over a selection resolving to more than one column spawns zero times, opens no title prompt, and calls `showError` exactly once with `dialog.bulk.oneRowOnly`; over a selection resolving to exactly one column it behaves as it does today.