---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:27Z
ordinal: 1
note: "The id is compared against the constant rather than against a second literal, so the manifest and the `registerMcpServerDefinitionProvider` call cannot drift; the API's own doc comment requires the two to agree. PLANT: change the manifest's id to `dinah.workbench` and the test must go red naming both spellings. SECOND PLANT: replace the label with a bare English string and the existing `l10n.test.ts:83` test, which requires every `%key%` to resolve, must not be the only thing that notices, so this criterion's own assertion must also go red. A false failure is not available, because both operands are read from files in the repository and neither depends on a machine."
---
`manifest.test.ts` asserts that `contributes.mcpServerDefinitionProviders` is exactly one entry, whose `id` equals `MCP_PROVIDER_ID` from `identity.ts` and whose `label` is a `%key%` reference resolving in `package.nls.json`, with the `%key%` half read from a raw `JSON.parse` of `package.json` rather than from the file's `manifest` binding, which `resolveNls` at `manifest.test.ts:84-103` has already replaced with the English.