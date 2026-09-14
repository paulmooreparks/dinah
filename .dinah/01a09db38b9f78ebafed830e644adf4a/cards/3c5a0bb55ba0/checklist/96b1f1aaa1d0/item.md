---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:56Z
ordinal: 20
note: Appending to the typed reference is simpler and it is wrong here. It would print pb-1/checklist/2 for a checklist item that every other surface prints as pb-1/questions/1, which breaks dinah-454's ruling that one entity has one printed spelling, and tree.go's own comment on containedNode already states that rule for show and contents. Extracting the per-member body of containedChildren is what makes the two callers unable to drift, and it moves the item counter with it, since an item's reference carries its position within its own kind rather than within the whole checklist. The collection's own Ref stays as typed, because the header should read back what the reader wrote.
---
A member's printed reference is composed by the containment walk's own composer rather than by appending a position to the reference the reader typed, and the composition moves into one function, Library.memberNodes, that the walk and the two collection-rooted commands share.