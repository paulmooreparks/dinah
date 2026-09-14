---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:28Z
ordinal: 19
note: "The documented route exists and is stable API: it first appears in the stable `index.d.ts` at `@types/vscode@1.101.0`, established by fetching 1.99.0, 1.100.0, 1.101.0 and 1.102.0 from unpkg and grepping (0, 0, 3, 3). Its doc comment says it provides servers \"in addition to those the user creates in their configuration files\" and returns a Disposable that \"unregisters the provider when disposed\". So the answer to \"which configuration surface is written, and at which scope\" is none, at no scope. The alternative, editing `.vscode/mcp.json` or the user-profile `mcp.json`, has no documented editor API for writing and would reach into a reader's own file for a value they did not ask for. Removal therefore reduces to disposing the registration, which the editor does on deactivate."
---
Registration goes through `vscode.lm.registerMcpServerDefinitionProvider`, and no file of the reader's is written, read or watched.