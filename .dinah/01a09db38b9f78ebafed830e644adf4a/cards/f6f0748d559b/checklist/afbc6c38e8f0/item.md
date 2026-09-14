---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:28Z
ordinal: 21
note: "Two reasons. The label is composed from the product's own name and the reader's own workbench data, so there is no sentence for a translator to render and no entry for `src/locales/*.json` to carry. And the label is documented only as \"The human-readable name of the server\", with nothing in `@types/vscode@1.101.0` or in the published documentation saying where the editor draws it or whether the editor keys any per-server state on it; a label that changed with the editor's display language would therefore change a string whose role the editor does not document, so a language-invariant label avoids resting on an answer nobody has published. The placement claim this note carried in round 1, that the label is what the editor's MCP server list shows, was withdrawn at Spec round 2 on the Agent Design Review finding that it asserts where VS Code draws a string. Nothing in the reasoning depended on it. The untitled fallback exists because `rootsFor` substitutes `UNTITLED_WORKBENCH`, spelled `\"Dinah\"` at `tree.ts:701`, which would render `Dinah: Dinah` and would be indistinguishable from a workbench genuinely titled Dinah. That is why `mcpTargets()` is a new accessor rather than a reuse of `rootsFor`."
---
The server label carries no authored English and so gains no runtime catalogue key; it is `Dinah: <workbench title>` with `Dinah: <basename of root>` for an untitled workbench.