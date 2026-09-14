---
kind: decision
state: resolved
ts: 2026-09-14T02:16:53Z
ordinal: 10
note: A pointer without a path repeats what section 1 already says in general terms, which is exactly what failed here. A pointer with a path is the next dinah-195 if the path moves and the document does not, so the path and the guard land together. The guard asserts both halves, that section 5.3 names the path and that the path exists, because the one-half version passes vacuously the moment somebody deletes the sentence.
---
The pointer names docs/design/format.md by path, and a guard test keeps the path honest.