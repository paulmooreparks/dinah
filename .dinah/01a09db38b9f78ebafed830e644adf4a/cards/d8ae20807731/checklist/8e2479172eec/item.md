---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:40Z
ordinal: 4
note: TestAnAmbiguousNumberNamesEveryCandidateInOrder builds the three-card variant with no archive and asserts the row count is three and the rows read c00000000001, c00000000002, c00000000003 in that order.
---
Three cards carrying one number produce three rows in ascending identifier order, and the test asserts the row count as well as the contents. The bench-package fixture in the spec is built with c00000000001, c00000000002, and c00000000003 all carrying number 1 and no archived card, and resolveCardIn against the live root refuses with the identifiers in that ascending order.