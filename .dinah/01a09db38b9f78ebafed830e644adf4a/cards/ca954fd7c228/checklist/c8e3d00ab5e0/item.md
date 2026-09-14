---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:24Z
ordinal: 18
note: "`{ prefix, module, table }` points at `HISTORY_ROWS` in `servedText.ts`, whose property names the guard reads off the AST, so the event names stay written in one place. The two hand-written fields cannot rot silently: a prefix matching no catalogue key fails, and a template head matching no family fails as unresolvable. It sits in the test file rather than in `src/` because it is build-time data no reader's text passes through and `src/` is bundled into `dist/extension.js`."
---
A key family declares where its members come from rather than listing them, and the declaration lives in the test file.