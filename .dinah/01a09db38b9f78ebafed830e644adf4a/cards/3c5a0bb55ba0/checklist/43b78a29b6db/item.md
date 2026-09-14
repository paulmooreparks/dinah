---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:56Z
ordinal: 27
note: "ResolveEntity deliberately leaves the workbench's Ref empty, calling its spelling a question that resolver does not settle, and both consumers already answer it for themselves: Library.Attachments substitutes bench.WorkbenchRef for its header, and Library.Contents seeds its children with the slug, per dinah-151 OQ-9. Filling Holder.Ref here would be a third answer, and it would be the wrong one for at least one of them, since the header wants `workbench` and the child addresses want the slug. The one case ResolveEntity cannot answer is a bare slug head, which names no entity to it and is a legal head below the workbench, so ResolveReference builds that holder itself as the workbench and nothing else changes."
---
CollectionRef.Holder is exactly what ResolveEntity answers for the entity the collection hangs from, with the workbench's own reference left empty, and the two consumers that need a spelling for the workbench keep the rules they already carry.