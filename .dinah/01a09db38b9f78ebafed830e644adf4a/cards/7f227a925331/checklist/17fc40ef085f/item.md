---
kind: decision
state: resolved
owner: holder
ts: 2026-09-14T02:18:44Z
ordinal: 24
note: A workbench declaring format 1 or 2 keeps its numbers in frontmatter and is read that way through one branch in LoadCardIn; `dinah add` refuses with `dinah.needs-number-migration` naming the flag, on the precedent of NeedsContainerMigration and NeedsVocabularyMigration. Refusing to open would leave an operator with no route to the migration that would open it, and writing the number to both stores during a grace period would be the two-stores hazard by another name.
---
An unmigrated workbench opens and reads, and only allocation refuses.