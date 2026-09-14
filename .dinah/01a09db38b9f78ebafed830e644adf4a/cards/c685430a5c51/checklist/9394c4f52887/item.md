---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:07Z
ordinal: 19
note: "Three cuts, each with its own reason. The archive mirror is left out because every other member of Bench.Check reads the live tree. The walk stops at a mountless kind rather than descending, because a directory below a stray is unreachable for the same reason the stray is and one finding names the whole of what an operator opens. The sweep asks only about attachments because attach is the only verb that can write below a mountless kind, which section 2 of the spec shows by running the call-site search over the tree: AddComment and AddItem are reached only through verbs that resolve a card and only a card. A generalised sweep would report the same directories at the cost of a listing per entity per mount."
---
The check walks the live tree only, reports one finding per stray directory, and looks for an `attachments` directory rather than for any collection directory a kind does not mount.