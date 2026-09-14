---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:17:33Z
ordinal: 24
note: A link carries no identity and no journal of its own (format.md), so the archive/delete distinction that exists for identity-bearing entities (cards, comments, attachments) has nothing to attach to here. `unlink` removes the frontmatter entry outright; the removal is still recorded via EventUnlinked on the card's own journal.
---
D-3: removing a link is a delete, not an archive.