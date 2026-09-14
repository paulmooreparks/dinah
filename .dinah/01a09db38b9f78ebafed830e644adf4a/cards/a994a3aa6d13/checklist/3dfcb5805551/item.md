---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:50Z
ordinal: 23
note: "PASS, with the arming the criterion demands. The guard at internal/mcp/mcp_test.go:650 now counts workbench alongside actor and basis (line 690) and asserts the injected total equals 3*len(tools)-1 (line 703), the minus one being the workbenches exception, which tools.go:150 guards by name.\n\nArmed by deleting the three-line injection at internal/mcp/tools.go:150: the test failed with \"read 64 injected properties across 32 tools\". 64 is exactly 2*32, which is the count the old weak form of this guard asserted, so this is direct evidence that the pre-strengthening guard would have passed with the injection missing entirely. Source restored, tree clean, suite green afterwards.\n\nOne reading recorded rather than decided quietly: the criterion's note says the strengthened assertion is 3*len(tools), while the shipped form is 3*len(tools)-1. The workbenches tool genuinely carries no workbench property, so a plain 3n would be wrong and the shipped form is the correct strengthening. Read as intent rather than literal arithmetic."
---
`TestEverySchemaPropertyIsDescribedAndNoneCarriesAnEnum` counts `workbench` as an injected property alongside `actor` and `basis`, and deleting the injection from `schemaFor` turns that test red.