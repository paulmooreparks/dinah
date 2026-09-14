---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:48Z
ordinal: 8
note: "Verified. Test: editors/vscode/test/unit/cardCommands.test.ts, \"a delete checkpoints the folder the row stands in, whichever way dinah answered\". Command: npm --prefix editors/vscode run test:unit.\n\nThe ok arm asserts deepEqual(checkpoints, [\"C:\\work\\bench\"]) and errors.length === 0. The refused arm answers the delete with refused(\"dinah.unknown-path\", \"tr-4/attachments/0b2c3d4e5f61\") and asserts the same checkpoint together with errors.length === 1 carrying refusalMessage's composition, \"dinah.unknown-path: tr-4/attachments/0b2c3d4e5f61\", since runVerb checkpoints on a refusal too and the read that follows is what shows the reader the board moved under them.\n\nArmed once. Plant: the `await context.host.checkpoint(context.folder);` line deleted from runVerb. The plant compiles and the run executed 540 tests. Red assertion: deepEqual(checkpoints, [\"C:\\work\\bench\"]) reporting + [] against - [ 'C:\\work\\bench' ]. Five existing tests reddened with it (the claim refusal, the whole-verb refusal sweep, the successful-call checkpoint, the move act, and the refused drop), which is the intended blast radius for a shared path.\n\nThe checkpoint asserted here is the whole of the extension's refresh obligation, and dinah-451 AC-10 pins the Go-side fact that makes it redraw anything."
---
One off-cycle checkpoint runs against the folder the row stands in, on a successful delete and on a refused one alike.