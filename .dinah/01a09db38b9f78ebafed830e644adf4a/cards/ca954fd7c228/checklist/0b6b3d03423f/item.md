---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:25Z
ordinal: 26
note: The two patterns differ, `/\{([A-Za-z0-9_]+)\}/` in the extension and `\{[a-zA-Z][a-zA-Z0-9_.-]*\}` in Go, and each guard uses its own side's. A guard whose idea of a placeholder differs from the renderer's checks a different thing from the one that ships, and the divergence would show up as a name the guard thinks is safe and the renderer leaves unfilled.
---
The guard's placeholder pattern is copied character for character from `fill` in `src/l10n.ts`, and the CLI's new test reuses the pattern its sibling test compiles.