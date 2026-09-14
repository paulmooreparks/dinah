---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:49Z
ordinal: 21
note: PASS, and the arming the criterion prescribes does work here. Unarmed, a tools/call naming a path outside the root holding no workbench.md answered dinah.outside-root with the root in context. Armed by moving the PathUnderRoot block after the workbench.md existence block in resolveLibrary (internal/mcp/mcp.go), the same call answered dinah.no-workbench. The guard can fail, so the per-call half of the declared order is genuinely held.
---
Against `dinah mcp --root <dir>`, a `tools/call` whose `workbench` property names a path that lies outside `<dir>` and holds no `workbench.md` answers `dinah.outside-root` rather than `dinah.no-workbench`.