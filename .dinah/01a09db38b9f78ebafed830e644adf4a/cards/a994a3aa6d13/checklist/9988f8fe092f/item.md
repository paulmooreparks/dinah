---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:49Z
ordinal: 13
note: "PASS, all three clauses. Run from a scratch directory where the ancestor walk finds nothing, with --root naming an empty directory: exits 0 and serves, stderr empty (so nothing was discovered and dropped), a workbenches call answers with an empty array, and a status call carrying no workbench property refuses dinah.no-workbench-found. The verifier confirmed no workbench.md exists at C:\\ or at C:\\dinah-scratch, so the walk genuinely found nothing rather than reaching live data."
---
`dinah mcp --root <an empty directory>` run where discovery finds no workbench serves rather than exiting, answers a `tools/call` of `workbenches` with an empty array, and refuses a `tools/call` of `status` carrying no `workbench` property with `dinah.no-workbench-found`.