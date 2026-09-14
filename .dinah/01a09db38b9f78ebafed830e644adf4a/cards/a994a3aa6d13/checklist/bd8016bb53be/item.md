---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:49Z
ordinal: 14
note: PASS. An initialize against a server bound to a root, started inside a workbench under it, answers with instructions whose first line is exactly `You are working the workbench wbA.`, names the --root directory in its reach sentence, and names the workbenches tool in the sentence following. Shown to be the root rather than the workbench's own path by comparing against the no---root run, where the same sentence names the workbench's store directory instead.
---
An `initialize` request sent to `dinah mcp --root <dir>` started inside one of the workbenches under `<dir>` answers with instructions whose first line reads `You are working the workbench <title>.` and which also name `<dir>` and the workbenches tool.