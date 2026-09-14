---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:30Z
ordinal: 13
note: The existing get/set field-write grammar. capacity is the direct precedent (typed name differs from stored key, Clearable, carries a Guard). dinah column's own doc comment says it authors only at creation, and get/set is the tool's one mechanism for reading/writing a declared column property afterward.
---
What shape does this command take: a new verb, a subcommand of `dinah column`, a flag on `dinah column new`, or a new field on the existing get/set grammar?