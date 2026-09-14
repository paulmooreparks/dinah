---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:48Z
ordinal: 3
note: "Verified. Test: editors/vscode/test/unit/tree.test.ts, \"an attachment row carries the contextValue its context menu is registered against\", driving attachingBench() to card ddd and taking both attachments of TWO_ATTACHMENTS (the first with a path, the second with none). Command: npm --prefix editors/vscode run test:unit.\n\nArmed once. Plant: tree.ts's attachment arm sets the key conditionally, `...(openable ? { contextValue: CONTEXT_ATTACHMENT } : {})`. The plant compiles and the run executed 540 tests. Red assertion: treeItemFor(spec).contextValue === CONTEXT_ATTACHMENT, reporting + undefined - 'dinah.attachment'. The assertion on `shot` sits ahead of it in the same test and stayed green, which is what shows the plant hit only the pathless row: exactly the failure D-4 exists to prevent.\n\nThe manifest half of the pairing is dinah-451 AC-2, which holds the same identity constant against the menu's own clause, so the row's spelling and the menu's spelling are checked against each other rather than each against a literal."
---
Every attachment row carries contextValue dinah.attachment, including one whose payload will not read.