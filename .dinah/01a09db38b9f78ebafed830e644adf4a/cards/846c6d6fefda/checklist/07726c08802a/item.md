---
kind: decision
state: resolved
column: c9428b3bc921
ts: 2026-09-14T02:17:11Z
ordinal: 9
note: RecognizedAt was itself the duplicated, independently-wrong copy of the question benchIn already answers correctly for every child directory. Fixing the shared function is what makes the MCP path (a second, real caller, confirmed by a live probe run) inherit the fix automatically instead of needing its own card.
---
The fix lives in bench.Enumerate (via enumerate's root probe), not duplicated per-caller; bench.RecognizedAt is removed since it had exactly one caller (confirmed by a whole-tree grep) and that caller's need is now met by Enumerate alone.