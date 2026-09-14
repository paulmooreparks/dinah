---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:17:44Z
ordinal: 18
note: "`dinah link <from> <to> <kind>` shipped with dinah-436, and format.md:1567 declares the kind an open enum whose suggested spellings are `duplicates` and `relates`. Since the enum is open and carries no behaviour on either side, a mapping would only cost a reader a translation step for no gain. The workbench's own standing text records that this board uses Andoneer's four, which keeps the choice where a reader meets it rather than in this card. The alternative, adopting Dinah's suggestions, was rejected because `parked_behind` and `supersedes` have no suggested counterpart at all and would have had to be minted anyway."
---
The cutover adopts Andoneer's four link spellings verbatim: blocks, relates_to, supersedes, parked_behind.