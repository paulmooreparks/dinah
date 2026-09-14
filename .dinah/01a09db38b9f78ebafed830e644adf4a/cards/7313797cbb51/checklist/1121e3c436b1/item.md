---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:25Z
ordinal: 14
note: All four were run and all four resolve to wb-1. They fall out of strconv.Atoi accepting a leading zero, splitRef cutting at the last dash so any prefix is a prefix, and resolveCardIn trimming the reference. Nothing writes them back, so refusing them would break references that work today and buy no reader anything. AC-6 turns the tolerance into a recorded property by pinning what Dinah answers with, which is the half that matters.
---
The resolver is not narrowed. `wb-01`, `wb--1`, `WB-1` and a reference carrying surrounding whitespace go on resolving, and the roster records them as tolerated spellings rather than as address forms.