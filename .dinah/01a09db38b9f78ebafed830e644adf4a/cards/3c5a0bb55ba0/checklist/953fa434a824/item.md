---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:56Z
ordinal: 26
note: "rankOfKind counts from the workbench, and fillContained compares a node's rank against the level's limit. Giving the collection root the holder's rank puts its members at the rank they occupy in a walk rooted at the holder, so `dinah contents pb-1/comments --depth cards` and `dinah contents pb-1 --depth cards` hide the same rows. Giving the collection a rank of its own would make a collection a level of the ladder, which the containment table does not say it is: a collection has no anchor, no identifier and no row in that table, and the depth ladder is a ladder of entities."
---
A contents walk rooted at a collection takes its root rank from the collection's holder, so --depth cuts that walk exactly where it cuts a walk rooted at the holder.