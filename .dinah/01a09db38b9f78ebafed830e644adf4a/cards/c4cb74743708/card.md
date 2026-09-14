---
title: A comment in the extension's catalogue overstates what the server fills in
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
workstreams:
  - 58f3e3eb621a
---
The comment at editors/vscode/src/verbCatalog.ts lines 52-53 tells a reader that the MCP server fills in all three of its injected values on every command it serves. Reading the running binary's own output says otherwise: of the 43 tools it serves, 8 carry all three injected properties and 34 carry two.

Nothing follows from this in behaviour. The code checks each property one at a time rather than assuming a fixed three, so it is correct as written; the comment is the only thing that is wrong, and no code path reads it. It is filed because it tells the next reader the surface is uniform when it is not, which is the sort of thing that turns into a wrong assumption two cards later.

The fix is one clause. Both the code review and the test stage on dinah-420 saw it and carried it forward rather than spending a round trip on a comment, so it is filed here instead of being lost in two review notes.
