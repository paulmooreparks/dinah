---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:28Z
ordinal: 22
note: "The field is documented purely as a change signal: \"If this changes, the editor will indicate that tools have changed and prompt to refresh them.\" The release number moves on every release, so it strictly dominates `profile`, which claims a contract and does not move when a binary gains a tool inside the same contract. `api.ts:11` says the release number is \"Displayed, never compared\", documenting the `tool` field on the line below it, and that rule forbids inferring compatibility between an extension and a binary from their two numbers; what happens here is the editor comparing one binary's own value against its own earlier value, which is change detection over one artifact rather than a compatibility inference across two, so the rule is untouched."
---
`McpStdioServerDefinition.version` is the binary's own release number, `binary.version.tool`.