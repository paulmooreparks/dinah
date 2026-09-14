---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:51Z
ordinal: 51
note: "`offerDrag`'s own doc comment states today's behaviour: a drag starting on a column, a state group, a workbench or an attachment row sets no entry, so the drop afterwards is indistinguishable from one this controller never handled. Under `dragRowsFor` the same gesture carries whichever cards the selection holds, and the drop is handled. That is right, because a reader who selected four cards and a column header and dragged the lot meant the cards, and it is declared here rather than left to be discovered because it changes shipped behaviour without any criterion naming it as a change. AC-17 requires the now-false sentence in `offerDrag`'s comment to be replaced in the same diff. A drag holding no card row at all still sets nothing, so the silent-miss behaviour survives for the gesture it was written for.\n\nWidened at round 5 from \"every card in the selection\" to \"every row in the selection\", which is D-22's consequence: the non-card rows cross the mime entry too, so the report can name the row it could not act on instead of pretending the reader dragged only cards. The condition on setting the entry is unchanged and still reads \"at least one card row\", and `dragRowsFrom` refuses a list holding no card row for the same reason, so the two halves of that rule cannot drift apart.\n\nRound 2's `dragPayloadsFor` is gone and D-14 records why; this note named it until round 3."
---
A mixed drag carries every row in the selection, and the mime entry is set whenever at least one of them is a card. Today it carries only the first dragged row when that row is a card, and sets no mime entry at all when it is not.