---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:03Z
ordinal: 2
note: "Planted and observed at c25c20f. Inserted `| pull | no | no | no | yes | no |` into the guide's table and ran the same test. It failed with `references_guide_test.go:133: the references guide's table carries a row for pull and no command of that name points at the guide`, plus the count line `the table draws 16 rows against a roster of 15`. Restored byte-identically, test ok."
---
Insert the row `| pull         | no          | no       | no     | yes          | no           |` into the guide's table and run the same test. It fails with `the references guide's table carries a row for pull and no command of that name points at the guide`. `pull` is a real command that takes a card rather than a reference, so the plant compiles and runs and is not a nonsense name.