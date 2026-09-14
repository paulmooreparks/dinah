---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:17:44Z
ordinal: 13
note: "Required. This is a defect in shipped machinery, not a scope choice: arrayFromChildren (internal/bench/blockjson.go) drops every field but the first on a multi-key dashed entry, so a citation's target and observed pair never read back, and CountCitations only counts, it does not decode. Already filed and moving in Build Queue as dinah-438, independent of this ruling. Layer: contract-core, internal/bench/blockjson.go, generalizing the existing reader rather than adding a citations-specific one."
---
Gap 2, citations readback (dinah-438): required or Andoneer habit?