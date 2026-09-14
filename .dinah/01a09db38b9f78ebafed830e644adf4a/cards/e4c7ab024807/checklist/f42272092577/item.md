---
kind: decision
state: resolved
column: 4b38abe7ebd5
owner: holder
ts: 2026-09-14T02:17:49Z
ordinal: 18
note: cardCommands.ts's openAttachment carried "the row carries no context menu either (Decision 4), and its plain click is the whole of what it offers", written under dinah-335. dinah-451 D-4 makes that false in the same file. The spec's file table does not name the comment, so this is a fix the card's own decisions force rather than one it asked for. The clause now reads that the plain click is the whole of what THIS HANDLER offers and points at deleteAttachment below for the menu, which keeps dinah-335's reasoning about openFile and loses only the sentence dinah-451 falsified.
---
openAttachment's doc comment loses the clause saying an attachment row carries no context menu.