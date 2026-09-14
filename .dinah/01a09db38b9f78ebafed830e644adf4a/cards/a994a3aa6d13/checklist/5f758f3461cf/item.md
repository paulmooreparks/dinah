---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:49Z
ordinal: 19
note: "PASS, all four clauses. Started from inside a workbench lying outside the root, with neither --workbench nor DINAH_WORKBENCH set: exits 0 and serves; stderr is exactly one line, `no default workbench: <path>`, which is the mcp.no-default template from en.json naming the workbench it dropped; a status call with no workbench property refuses dinah.no-workbench-found; a status call naming a workbench under the root answers with that workbench's status. This is the half of startup case 3 that must not exit, because discovery climbs from the client's chosen working directory and a head exiting here would start on one machine and refuse on another with the same registration."
---
`dinah mcp --root <dir>` started from inside a workbench that lies outside `<dir>`, with neither `--workbench` nor `DINAH_WORKBENCH` set, serves rather than exiting, writes one line to stderr from `mcp.no-default` naming the workbench it dropped, answers a `tools/call` of `status` carrying no `workbench` property with `dinah.no-workbench-found`, and answers a `tools/call` of `status` naming a workbench under `<dir>` with that workbench's status.