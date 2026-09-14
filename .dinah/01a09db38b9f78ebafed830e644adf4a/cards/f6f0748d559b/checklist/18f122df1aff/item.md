---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:29Z
ordinal: 23
note: "The default is true because the operator ruled that the extension registers; the setting exists because a registration the reader cannot decline is past what was approved. Scope is `window` rather than the `resource` that `dinah.workbench`, `dinah.pollIntervalSeconds` and `dinah.watchFiles` carry, because `provideMcpServerDefinitions` answers once for the whole window with one flat array and the published set is deduplicated across folders: a root two folders both reach would need a reconciliation between two folder-scoped answers, and there is no defensible one. `dinah.path` is already off the folder axis, at `machine-overridable`, for the same kind of reason. The read is `settingOf` with a boolean type argument and a `true` fallback, made on every call rather than once at activation, so the setting is honoured on the next call the editor makes. Round 3 moved where that read sits: it is in the `currentPlans` helper the provider callback invokes, and its value is passed into the pure `publishedMcpServers`, because the decision the setting feeds cannot live in a module no unit test can load."
---
`dinah.registerMcpServer` is a boolean defaulting to true, at `window` scope.