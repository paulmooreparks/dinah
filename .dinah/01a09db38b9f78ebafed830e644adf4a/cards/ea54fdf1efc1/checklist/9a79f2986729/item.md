---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:10Z
ordinal: 15
note: The four existing `*_updated` events carry `from` and `to`, and copying them onto a prose write would put a whole instructions body, card body or comment body into an append-only journal on every edit, twice over for a rewrite. The journal records that an act happened and who did it; the anchor is the file that changed and git carries its history. `Field.Prose` is the declaration that decides, so the rule is read off the same table everything else about a field is read off rather than out of a second list. AC-7 fails a line carrying `to` on a prose write.
---
A write to a prose field journals the act and not the prose: the line carries `field` and the entity's name and carries neither `from` nor `to`.