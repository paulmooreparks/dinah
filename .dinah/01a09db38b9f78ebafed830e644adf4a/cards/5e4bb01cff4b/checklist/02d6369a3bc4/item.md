---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:17:00Z
ordinal: 3
note: "Test-stage re-verification by independent Python sweep of en.json on the merged tree (word-bounded, case-insensitive, matching the guard's own {..} + backtick + <angle> + --flag stripping and \"never translated\" context exclusion): \"the root\" triggers exactly 4 keys (check.mcp.2, mcp.reach, mcp.reach.nodefault, refusal.dinah.outside-root); \"state\" triggers 78 with brace-stripping alone and 77 with the full strip (the one key removed is refusal.dinah.ambiguous-state.next). Matches this criterion's counts exactly."
---
The glossary's "root" term triggers on exactly check.mcp.2, mcp.reach, mcp.reach.nodefault and refusal.dinah.outside-root, and not on param.contents.depth.summary, param.tree.depth.summary, check.mcp.1, init.done, status.workbench, refusal.dinah.outside-root.next or refusal.dinah.unknown-root.next; the "state" term triggers on all English entries containing the bare word "state" except param.tree.group-by.summary (context-excluded as declared "never translated"), card.line, card.line.workstreams, and refusal.dinah.no-upstream (each carries "state" only inside a {state} placeholder that placeholder-stripping removes before matching).