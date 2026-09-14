---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:27Z
ordinal: 3
note: Re-verified at Test by running the built binary, both spellings. `dinah set wb1-1/criteria/1 column Done` (title) exited 0 and stored b50584c8a63c (Done's id); `dinah set wb1-1/criteria/1 column intake` (slug) exited 0 and stored f23d3cb83d48 (Intake's id). Both read back via `dinah get`.
---
Running the built binary: dinah set <item> column <a column's slug or title> succeeds (exit 0) and stores the column's identifier on disk (read the item's anchor file or dinah get <item> column), matching what File already does.