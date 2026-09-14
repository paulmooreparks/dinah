---
kind: acceptance_criterion
state: pending
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:19Z
ordinal: 3
---
If the chosen response adds a field to the moved event: a fixture-replay test extending populate.txt produces one moved line for an adjacent move and one for a jump, and asserts the new field's value differs between the two lines in the way the response specifies. A run where both lines carry the same value for the new field fails this criterion. (Not applicable, and satisfied vacuously with a one-line note saying so, if the chosen response adds no journal field.)