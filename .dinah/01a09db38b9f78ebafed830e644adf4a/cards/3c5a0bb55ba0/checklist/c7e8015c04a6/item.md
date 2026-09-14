---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:56Z
ordinal: 22
note: "The parent's table gives attachments \"yes for attachments, empty listing for any other collection\". Answering from the holder rather than re-composing a listing of its own is what makes the two spellings one answer, and it is one line: the branch sets the entity to the collection's holder and the existing body runs. The empty half is not a new posture either. Library.Attachments already answers an empty listing for an entity whose kind mounts no attachments, and its own doc comment gives the reason, which is that a caller walking a tree asks the same question everywhere rather than deciding first whether the question is legal."
---
`dinah attachments <head>/attachments` answers from the holder, so it prints what `dinah attachments <head>` prints, and every other collection reference answers an empty listing whose kind is collection.