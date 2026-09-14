---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:48Z
ordinal: 5
note: "Verified. Test: editors/vscode/test/unit/cardCommands.test.ts, \"deleting an attachment addresses it by its identifier and not by its position\", driving the new deletableRow() fixture (owner \"tr-4\", root \"C:\\work\\bench\", view id \"0b2c3d4e5f61\", ordinal 2, ref \"tr-4/attachments/2\", filename \"spec.pdf\") with the recorder's confirmation answering true. Command: npm --prefix editors/vscode run test:unit.\n\nAssertions: calls.length === 1, and a deepEqual of calls[0] against [\"--json\", \"--workbench\", \"C:\\work\\bench\", \"delete\", \"tr-4/attachments/0b2c3d4e5f61\", \"--yes\"].\n\nArmed once, on the second attempt. The first plant, replacing the composition with `element.view.ref`, left ATTACHMENTS_SEGMENT unread and failed tsc with TS6133, which printed no test output at all and would have read exactly like a pass; that is the trap the workbench's own instruction names, and it is recorded here rather than quietly retried. The plant that ran replaces the identifier with the position, `${element.owner}/${ATTACHMENTS_SEGMENT}/${element.view.ordinal}`, which compiles and executed 540 tests. Red assertion: the argv deepEqual, reporting + 'tr-4/attachments/2' against - 'tr-4/attachments/0b2c3d4e5f61'.\n\nThe fixture's identifier and position differ on purpose, which is what makes the criterion distinguish rather than agree with itself."
---
A confirmed delete spawns exactly one dinah call, addressing the attachment by identifier and carrying --yes.