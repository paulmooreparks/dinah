---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:49Z
ordinal: 16
note: PASS, both halves. With DINAH_MCP_ROOT naming one directory and --root another, a workbenches call listed only the two workbenches under the directory --root named, and the one under the variable's directory was absent. With the variable set and the flag absent, the same call listed the variable's workbench, so the variable is read rather than ignored. The flag wins and the variable works, which is the ladder --workbench and DINAH_WORKBENCH already climb.
---
With `DINAH_MCP_ROOT` set to one directory and `--root` naming another, a `tools/call` of `workbenches` lists the workbenches under the directory `--root` named and none of the others.