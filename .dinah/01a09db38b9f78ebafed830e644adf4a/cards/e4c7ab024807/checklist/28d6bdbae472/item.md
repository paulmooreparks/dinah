---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:48Z
ordinal: 7
note: "Verified. Test: editors/vscode/test/unit/cardCommands.test.ts, \"the confirmation names the file and the address the row was drawn from\". The recorder records both arguments every confirmDestructive call was made with. Command: npm --prefix editors/vscode run test:unit.\n\nAssertions, both equalities against the same ENGLISH localizer the file already imports: the message against ENGLISH(\"dialog.attachment.delete.confirm\", { filename: \"spec.pdf\", ref: \"tr-4/attachments/2\" }), and the label against ENGLISH(\"dialog.attachment.delete.action\"). Equality rather than containment, because a containment check goes on passing while the placeholders are filled from each other's fields.\n\nArmed once. Plant: the two placeholder values swapped at the call site, so filename receives view.ref and ref receives view.filename. The plant compiles and the run executed 540 tests. Red assertion: the message equality, printing + 'Delete tr-4/attachments/2 (spec.pdf)? Dinah destroys the file and cannot bring it back.' against - 'Delete spec.pdf (tr-4/attachments/2)? Dinah destroys the file and cannot bring it back.'\n\nA false failure would need ENGLISH to render differently between the call site and the test, and both read the same catalogue entry."
---
The confirmation names the file being deleted and the address the row was drawn from, with the two placeholders filled from the right fields.