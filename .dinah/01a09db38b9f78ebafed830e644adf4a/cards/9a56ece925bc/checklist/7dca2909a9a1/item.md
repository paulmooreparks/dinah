---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:18Z
ordinal: 12
note: "The card asked whether to refuse or to reach something inside. A workstream directory holds two files, `workstream.md` and `journal.ndjson`, and the attachments the ambiguity argument rests on do not exist: `dinah attach workstream/<slug> <file>` refuses `dinah.not-attachable` and `dinah contents workstream/<slug>` reports that it contains nothing, both observed at 808d105. The anchor is the obvious target because every other kind `edit` accepts opens its own anchor, and because the anchor carries the workstream's `notes` field, which `dinah get` and `dinah set` already read and write. The journal is machine-written ndjson, and a card's journal is reached by its own segment where a reader wants it, a spelling that does not exist below a workstream. Refusing would also make `edit` the one command answering a real entity with a refusal while `path`, `get` and `set` answer it, and would force an edit to the references guide's workstream sentence."
---
`edit` opens a workstream's anchor rather than refusing the reference.