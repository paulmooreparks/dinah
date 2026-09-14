---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:49Z
ordinal: 12
note: PASS. `dinah --workbench <workbench outside the root> mcp --root <dir>` exits 2 with `dinah.outside-root` as the first whitespace-delimited token on stderr and nothing on stdout. This is the contradiction-in-the-registration case, kept distinct from AC-19's discovery case, which serves.
---
`dinah --workbench <a workbench outside the root> mcp --root <dir>` writes `dinah.outside-root` as the first whitespace-delimited token on stderr and exits 2.