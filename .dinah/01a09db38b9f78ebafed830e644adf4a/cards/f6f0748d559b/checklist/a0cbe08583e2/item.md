---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:27Z
ordinal: 6
note: "The count is computed from the fixture's answered rows rather than written as a literal, so the assertion cannot be satisfied by a walk that read nothing: a sweep returning an empty array against a fixture with three answered rows fails, which is the vacuous-sweep shape the workbench instructions name. The fixture must therefore carry at least one row of each kind, and the test asserts the fixture's own answered count is greater than zero before comparing. PLANT: make `mcpTargets` return every row regardless of `data.fetchedAt` and the candidates assertion must go red with a target whose root is a candidate path. SECOND PLANT: return an empty array unconditionally and the count assertion must go red rather than reading as a clean sweep."
---
`tree.test.ts` asserts that `mcpTargets()` returns a target for a folder in `single` and in `forest` mode, none for a folder in `candidates`, `dead-end` or unanswered state, and that the returned count for a mixed fixture equals the number of fixture rows that are not `deadEnd` and carry `data` with a non-empty `data.path` and a set `data.fetchedAt`, a number the test computes from the fixture and asserts is greater than zero. `holdingReportOf` is not exported, so that expected count is the test's own restatement of the predicate and no assertion here touches the function.