---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:00Z
ordinal: 24
note: "Verified that `dinah contents workbench` prints `workbench` for the root and `pb/attachments/1` for a workbench attachment on the same screen, and that both resolve. Changing the seed the children compose against would rename every address below the workbench and desync the printed form from the resolver's own default, which is a larger change than the inconsistency costs. The guide states plainly that the workbench has two spellings and both resolve, which is what the address rule actually requires: a reader can type what they see either way."
---
The workbench keeps two printed spellings, `workbench` for its own row and the slug for the addresses below it, and dinah-151 OQ-9 is not reopened.