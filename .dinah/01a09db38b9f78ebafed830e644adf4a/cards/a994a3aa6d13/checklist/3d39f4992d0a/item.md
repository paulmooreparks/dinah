---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:50Z
ordinal: 22
note: PASS, both spellings against one running server bound to a relative root. Naming a workbench by its absolute path and naming the same workbench by a relative path both answered with the same status and the same absolute root. A second workbench named by a dot-relative path answered correctly too. Stderr empty, exit 0. This is the criterion that holds the root's filepath.Abs and each call's filepath.Abs to the same base, which nothing else on the card checks.
---
`dinah mcp --root <a relative path>` started in a directory whose relative path reaches the root answers a `tools/call` naming a workbench under that root by its absolute path with that workbench's status, and answers a `tools/call` naming the same workbench by a relative path the same way.