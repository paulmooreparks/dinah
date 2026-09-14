---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:30Z
ordinal: 15
note: "It clears the gate_items key rather than writing gate_items: false. bench.go's strict parser already reads an absent key and an explicit false identically, so this never creates two on-disk spellings of \"off,\" and it reuses the existing Clearable/writeField machinery rather than a second write path."
---
Does "off" write gate_items: false, or clear the key?