---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:28Z
ordinal: 11
note: "This is dinah-473's own ruling for File, applied to the field rather than to one command. Both write paths run the same resolve-and-store-identifier discipline: File already does it, and this card adds a GuardColumnRef guard to internal/bench/fields.go's ItemColumnField declaration so `dinah set <item> column <value>` does too, via admitFieldValue refusing an unresolvable value and a routing branch resolving a resolvable one to its ID before writeField stores it."
---
A legal value is anything bench.ColumnByRef resolves (identifier, slug, or title), and both write paths store the resolved identifier, never the typed spelling.