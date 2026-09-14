---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:48Z
ordinal: 6
note: "Verified. Test: editors/vscode/test/unit/cardCommands.test.ts, \"a declined confirmation deletes nothing and asks dinah nothing\", on the same deletableRow() fixture with the recorder's confirmation answering false. Command: npm --prefix editors/vscode run test:unit.\n\nAssertions: deleteAttachment returned undefined, calls.length === 0, checkpoints.length === 0, errors.length === 0. The second half drives the dismissal path with a second recorder whose confirmation answers false for the reason the bound host reads an undefined showWarningMessage answer as declined, and asserts calls.length === 0 and checkpoints.length === 0 again.\n\nArmed once. Plant: the runVerb call moved above the confirmation in deleteAttachment, keeping the confirmation and its read in place so the module still compiles; the run executed 540 tests. Red assertion: assert.equal(r.calls.length, 0) reporting 1 !== 0, with checkpoints behind it in the same test.\n\nThis is the criterion that stops the modal from being decoration. A dialog shown after the act, or one whose answer is never read, looks identical in a screenshot and identical in review."
---
A declined confirmation deletes nothing, spawns nothing and runs no checkpoint.