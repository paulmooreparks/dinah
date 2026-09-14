---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:07Z
ordinal: 16
note: "`internal/verb/checks.go:225` says the reference and the file both resolve as row 1 and the owner as row 2, and `Library.Attach` checks the reference, then the owner, then the file. `dinah help attach` prints that order today, so the shipped page misstates it. Inserting the kind row in the middle would leave one row spanning two positions with a third between them, so the list is split into a reference row, the owner row, the kind row, and a file row. The cost is one English edit at check.attach.1 across eight catalogs and a decision record for German and Hindi."
---
The attach precondition list is split from two rows into four, which corrects a row that is already wrong rather than adding one to it.