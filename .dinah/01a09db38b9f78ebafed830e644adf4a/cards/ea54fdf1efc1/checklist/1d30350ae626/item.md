---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:11Z
ordinal: 26
note: "Read at 9260a2a, `toolExemptions` in internal/mcp/tools.go carries nine entries, path, edit, init, extract, reshape, config, mcp, guide and help, and the four grounds still cover them exactly as the parent's table said at 22a35fc: five on shell-or-filesystem, one on machine-not-workbench, one on the-head-itself, two on protocol-serves-it. Nothing this card adds needs a fifth. On the refusal side, every condition `get` and `set` can meet already has a name: the resolver raises `dinah.unknown-path` and `dinah.is-a-collection`, the field check raises `dinah.unknown-field`, the guards raise what the verbs they route to already raise, and a value that is absent or malformed raises `malformed`. Four keys and one splice are added to the catalogues, and no new token joins `internal/contract`. A refusal name is a compatibility commitment, so not minting one is worth saying out loud rather than leaving a reviewer to check."
---
No refusal name is minted by this card, and the four exemption grounds dinah-456 declared were re-checked against the nine entries at today's trunk rather than taken from the parent's count.