---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:48Z
ordinal: 13
note: AttachmentView.ref is composed by attachmentRef (internal/verb/read.go:1024-1029) as <owner-ref>/attachments/<n>, where n is displayOrdinal's index into the collection sorted by SortByOrdinal. memberPosition's own comment (internal/verb/read.go:1005-1012) says the position and the stored ordinal "stop coinciding after one delete". A row drawn before another session deletes an earlier attachment and attaches a new one would therefore address a different file, with nothing refused. pick (internal/bench/resolve.go:368-376) tests IsID(selector) ahead of the positional arm, and IsID (internal/bench/storage.go:111-124) accepts exactly twelve lowercase hex characters, so <owner>/attachments/<id> resolves to one attachment for as long as it exists. AttachmentView.ID is json:"id" with no omitempty (internal/verb/read.go:631-633), so it is present on every attachment a listing reports. The modal still shows view.ref, because that is the address a person could type.
---
The argv addresses the attachment by its 12-hex identifier, while the confirmation shows the positional reference.