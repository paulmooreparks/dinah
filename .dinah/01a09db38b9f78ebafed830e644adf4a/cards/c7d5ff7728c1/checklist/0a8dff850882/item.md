---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:00Z
ordinal: 20
note: Verified that `dinah restore` refuses dinah.unknown-command at 22a35fc, and that after archiving a comment the entity has no address at any surface. docs/design/format.md already declares the restored event, the note it carries and the structural act, and says in its own words that no command writes it, so this implements a published contract rather than minting one. Restore needs an address to take, which is why --archived reaches show, path and contents too; Bench.ResolveArchivedCard already does this for cards alone.
---
`restore` is added as the inverse of `archive`, and `--archived` generalises from cards to the whole of DinahPath.