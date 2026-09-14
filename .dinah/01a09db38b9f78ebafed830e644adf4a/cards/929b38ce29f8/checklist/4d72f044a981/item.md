---
kind: open_question
state: resolved
column: aa6cd1c6ae5f
owner: operator
ts: 2026-09-14T02:16:54Z
ordinal: 25
note: Index row should be widened.
---
CORE-INSTR-6 forbids writing the text of one instruction layer into another and is generic over instruction layers, but its row in the section 11 index gives the observable outcome as "After serving, neither the workbench's nor the state's stored instructions carry the other's text", which is scoped to the two layers section 7 names. The new section 7 paragraph relies on the generic reading, so a tool violating the prohibition with a third layer of its own breaks the statement while passing the indexed check. Should the index row be widened, or is the narrow check deliberate?