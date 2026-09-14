---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:47Z
ordinal: 2
note: "Verified. Test: editors/vscode/test/unit/manifest.test.ts, \"Delete Attachment is offered on an attachment row and on no other row in the tree\", reading the clause with the new soleClauseFor(\"1_attachment@1\") and feeding it to the existing opensOn helper. Command: npm --prefix editors/vscode run test:unit.\n\nArmed three times, every plant compiling and every run executing 540 tests.\n1. Plant: the manifest clause widened to viewItem =~ /^dinah\\./. Red assertion: the table's false row for CONTEXT_CARD_READY_CLAIM, reporting \"dinah.card.ready.claim true !== false\". The remaining twelve false rows are behind it in the same loop.\n2. Plant: the clause narrowed to viewItem == dinah.card.active. Red assertion: the true row, reporting \"dinah.attachment false !== true\".\n3. Plant: a second entry (openAttachment) added to the 1_attachment group. Red assertion: soleClauseFor's own length check, reporting \"1_attachment@1 holds 2 items 2 !== 1\".\n\nThe clause is read out of package.json rather than restated, so a table agreeing with itself cannot pass. Restored with git checkout -- after each plant."
---
The Delete menu entry opens on an attachment row and on no other row the tree draws.