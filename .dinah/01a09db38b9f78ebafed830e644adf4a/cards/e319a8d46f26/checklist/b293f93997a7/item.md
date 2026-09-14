---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:46Z
ordinal: 1
note: "Compares the PropertyAssignment node's initialiser kind in extension.ts's AST as parsed at test time, not a string in a file. Red when the property is absent, when it is `false`, or when the createTreeView call count is not 1. It cannot fire against correct code, because the only way to satisfy it is the property actually being present and true at the one call site. The limit, stated so nobody credits it with more: it proves the source says so, not that VS Code honoured it. Only the integration suite could prove that, and this card does not run it. The repo already sweeps src/ this way in test/unit/l10n-keys.test.ts and test/unit/l10n-coverage.test.ts."
---
A unit test walks `editors/vscode/src/extension.ts` with the TypeScript compiler API, finds every call whose callee text is `vscode.window.createTreeView`, asserts it found exactly one, and asserts that call's options object literal carries a `canSelectMany` property whose initialiser is the `true` keyword.