---
kind: decision
state: resolved
owner: holder
ts: 2026-09-14T02:18:44Z
ordinal: 19
note: docs/design/format.md:178 says collections get no ceremony files and an absent collection directory means an empty collection, so a file inside `cards/` would be the format's first exception to that rule. The numbering also spans both halves of the collection, and a file inside the live half would have to be explained as speaking for `archive/cards/` too. The name carries the entity kind because every other numbering in this format is a per-collection ordinal stored on the entity, so a bare `numbers.txt` would not say which numbering it holds.
---
The registry is `card-numbers.txt` at the workbench root, not `cards/numbers.txt`.