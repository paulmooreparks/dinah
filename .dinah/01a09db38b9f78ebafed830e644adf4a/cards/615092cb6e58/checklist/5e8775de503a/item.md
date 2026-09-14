---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:15Z
ordinal: 14
note: "The four commands resolve a reference to one thing, so merging the halves would have to count positions over the merged set, and archiving an unrelated entity would then change what an existing address means. search scans a set and each hit already carries its own Archived boolean, verified in Library.Search at 9260a2a, so admitting the mirror there hides nothing. The two behaviours are one sentence rather than two rules: --archived admits the mirror, and a command resolving a reference admits it by resolving in it where a command scanning a set admits it by scanning it too. The references guide carries that sentence. search's parameter keeps its own summary key rather than joining the shared one, so the two senses are written separately for a reader and for a translator."
---
A read under --archived shows the archived half alone on show, path, contents and restore, while the same flag on search goes on scanning both halves.