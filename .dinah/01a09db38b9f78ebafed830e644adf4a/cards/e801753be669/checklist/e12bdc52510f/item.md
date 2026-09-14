---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:17:33Z
ordinal: 27
note: format.md's own YAML example stores an id under `to:`, not a reference. resolveLinkTarget (new helper) tries the id form, then ResolveCard, then ResolveArchivedCard, mirroring HasIdentifier's own two-half scope.
---
D-6: the `to` argument resolves against both halves of the collection and stores the resolved identifier, never the caller's typed spelling.