---
kind: open_question
state: resolved
column: c9428b3bc921
owner: operator
ts: 2026-09-14T02:16:36Z
ordinal: 19
note: "Reviewer left AC-10 untouched on the operator's instruction. This question records the state discrepancy rather than resolving it: the dispatch brief described AC-10 as pending, and the board has it verified.\n\nPaul: Resolved"
---
Should AC-10 stay verified, or go back to pending until the shared roster obligation is discharged? The criterion has two halves. Its assertion about the code is true today and was verified. Its second half is a merge-time obligation, that whichever of PR #122 and PR #123 lands second adds "format" to the roster in TestParseArgsRecordsNoDomainCaptureForASessionFlag. With the criterion verified, the Test gate will not hold the card, so that obligation is carried only by prose inside an item nobody downstream is required to re-read. Raised at Agent Code Review cycle 6, which did not touch the criterion.