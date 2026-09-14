---
kind: decision
state: resolved
column: 4b38abe7ebd5
owner: holder
ts: 2026-09-14T02:18:28Z
ordinal: 15
note: "internal/bench/check_test.go's writeItem helper set the frontmatter key column to the value pending, which is an item state rather than any column the fixture declares. The new sweep reported it and TestOrdinalMigrationReplaysTheJournalAndIsIdempotent went red on its own assertion that a migrated workbench checks clean. The fixture was wrong rather than the sweep, so the key was corrected to state: pending, which is what the value plainly meant. The alternative, exempting the fixture, would have left the sweep unable to fire on the one workbench the package checks for cleanliness."
---
An existing bench test fixture stored column: pending on a checklist item, which named no column, and was corrected to state: pending rather than being exempted from the new sweep.