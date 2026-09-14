---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:48Z
ordinal: 6
note: PASS, checked on all 32 tools rather than sampled. Each of the 31 non-workbenches tools carries inputSchema.properties.workbench of type string with a non-empty description and no enum, and none lists it in `required`. All 31 descriptions are byte-identical, one schema across the surface. The `workbenches` tool carries no workbench property; its properties are actor and basis alone. No exceptions.
---
In that same `tools/list` answer, every tool except `workbenches` carries a `workbench` property of type string with a non-empty description and no enum, and the `workbenches` tool carries none.