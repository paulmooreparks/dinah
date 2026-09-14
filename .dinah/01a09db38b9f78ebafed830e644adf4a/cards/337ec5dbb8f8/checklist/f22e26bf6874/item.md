---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:03Z
ordinal: 12
note: "Both plants observed at c25c20f. Restoring the header cell \"This workbench\" failed with `references_guide_test.go:456: the references guide's table draws a line of 83 columns, and the table is written to fit eighty: \"| Command      | This workbench | A column | A card | Below a card | A collection |\"`. Restoring it and deleting the `| reopen |` row failed with `references_guide_test.go:460: this check measured 16 table lines, and the table draws a header, a separator and one row per command, which is 17`. Both restored byte-identically, test ok. TestAGuideTableSurvivesTheWindowItIsReadIn was updated to the new header, the new separator and the path and reopen rows, and it goes on measuring nothing."
---
Change the table's first header cell from `A workbench` back to `This workbench`, touching no other cell, and run `go test ./cmd/dinah/ -run TestTheReferencesGuideTableFitsAnEightyColumnWindow`. It fails with `the references guide's table draws a line of 83 columns, and the table is written to fit eighty`. Restore it, delete the `| reopen | ... |` row, and it fails with `this check measured 16 table lines, and the table draws a header, a separator and one row per command, which is 17`. Both plants were run against the draft table at b825059 with a probe carrying that test's body. `TestAGuideTableSurvivesTheWindowItIsReadIn`, updated to the new header and two rows, is a separate check: it proves the rows survive a 40-column read and measures no width.