---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:21Z
ordinal: 22
note: "Round 2 ruled the file untouched, and the sixth kind reopens that for one line. Line 127 names eight commands as taking a workstream and `restore` takes one, measured at ab5debc: `restore workstream/astream` exits 2 with dinah.not-archived on a fresh workbench, and exits 0 after `archive workstream/astream`. Deriving a workstream column from a list known to be short by one would encode the error in the declaration, so the sentence is corrected first. The rewrite stays on one line and the table is not given a sixth column, because `cmd/dinah/testdata/prose-figures.txt` pins four of this file's figures by line number at lines 18, 104, 127 and 129, derived by `grep -c '^internal/guide/guides/references.md:' cmd/dinah/testdata/prose-figures.txt`, and anything that re-wraps or inserts moves the last of them. The ledger's own entry at line 77 carries `figure=Eight` for that sentence and becomes `figure=Nine` on the same line. Round 1 said the ledger pinned three figures and that count was wrong; four is the derived figure."
---
`internal/guide/guides/references.md` is edited on exactly one line, its workstream sentence at line 127, and the promise sentence under dispute stands verbatim.