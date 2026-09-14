---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:28Z
ordinal: 13
note: card.ColumnTiers resolves its column reference on every read via ColumnByRef and never had this bug; GatingItems compares item.Column == columnID by raw string equality on purpose (dinah-450's strict-identifier discipline). Widening that comparison is a change to the gate's own contract, not to the field, and is out of scope for this card.
---
GatingItems' identifier-equality comparison in internal/bench/entity.go is left unchanged; the fix is entirely on the write side.