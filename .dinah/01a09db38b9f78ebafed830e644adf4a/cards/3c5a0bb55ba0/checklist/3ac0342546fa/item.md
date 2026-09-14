---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:55Z
ordinal: 16
note: "Reproduced at b825059: on a card that has never carried a comment, every command answers `dinah.unknown-path nothing in this workbench answers to C:\\...\\cards\\<id>\\comments`, which both denies a collection the workbench declares and leaks an absolute path into advice. That is the reported defect twice over, and it is the commonest case rather than an edge, since most cards carry no comment. Leaving the guard would ship a fix that only reaches collections which already hold something. This is the one place this card changes path, whose row in the parent's table reads \"unchanged from today\", and the change is confined to a reference that used to refuse: nothing that resolved before stops resolving, and a positional selector still refuses because ListIDs answers nil for a directory that is not there. AC-5 verifies both directions."
---
A collection the containment table declares resolves whether or not its directory has been created, so the Exists guard on descend's collection branch goes and `dinah path pb-2/comments` answers rather than refusing.