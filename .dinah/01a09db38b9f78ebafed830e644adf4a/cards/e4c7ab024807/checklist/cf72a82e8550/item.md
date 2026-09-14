---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:49Z
ordinal: 14
note: "The tool's own answer to this question is a required --yes marker (internal/verb/definition.go:342-345), which asks nobody anything, so the extension chooses its own shape. CommandHost gains confirmDestructive(message, confirmLabel) bound to the documented three-argument showWarningMessage(message, MessageOptions, ...items) with modal: true; the editor supplies Cancel and a dismissal answers undefined, which reads as declined. Two arguments rather than one because both strings are prose a reader meets. The message is \"Delete {filename} ({ref})? Dinah destroys the file and cannot bring it back.\" and the button is \"Delete\". It names the file because an attachment row's label is the filename alone, and it names the address because one entity can carry several attachments and two of them can share a filename. The owner half of that address comes from AttachmentListing.ref rather than from the attachmentsGroup element's ref: Library.Attachments (internal/verb/read.go:937-948) substitutes the literal \"workbench\" for the workbench's own empty reference, while the group carries \"\" (tree.ts:1799-1809, asserted at tree.test.ts:1499), which composes nothing resolvable."
---
The confirmation is a VS Code modal warning carrying two localised strings, and the attachment element carries the owner reference the listing resolved.