---
kind: decision
state: resolved
column: c9428b3bc921
ts: 2026-09-14T02:17:13Z
ordinal: 11
note: "TestTakesWorkUpAnswersForEveryKind already enumerates the full case table this rule turns on: a plain work column and an unimplemented \".\" kind (read as work per CORE-STATE-12) both answer true and declare all three states; an intake column, a done column, a dinah.buffer, and a work column with AwaitingOutside: true all answer false and, after this fix, declare none. States()'s only change is which slice it returns for the false side of that one boolean."
---
Every column kind's answer is stated once, off the existing TakesWorkUp() table, rather than branching per kind a second time.