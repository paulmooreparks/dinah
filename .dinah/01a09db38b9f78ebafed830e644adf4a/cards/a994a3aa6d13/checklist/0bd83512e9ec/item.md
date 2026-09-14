---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:49Z
ordinal: 18
note: "PASS, checked against today's tree so it accounts for the later amendments. The twelve keys, taken from this card's merge commit 12ea2f8 against the locale files: check.mcp.1, check.mcp.2, mcp.no-default, mcp.reach, mcp.reach.nodefault, param.mcp.root.summary, refusal.dinah.outside-root, refusal.dinah.outside-root.next, refusal.dinah.unknown-root, refusal.dinah.unknown-root.next, schema.workbench.description, tool.workbenches.summary. Exactly twelve, including mcp.no-default as the criterion says. The two changed keys are refusal.dinah.no-workbench and refusal.dinah.no-workbench-found. All fourteen are present with non-empty text in all eight locale files (af, cs, de, en, es, fil, hi, id); no gaps. This holds after dinah-211 restored the German catalog this card had regenerated, and after dinah-245 and dinah-248. `go test ./internal/msg/` passes."
---
Every locale file under `internal/msg/locales/` carries each of the twelve new catalog keys and the two changed ones, and `go test ./internal/msg/` passes.