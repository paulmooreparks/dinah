---
kind: decision
state: resolved
column: c9428b3bc921
ts: 2026-09-14T02:17:19Z
ordinal: 10
note: 262,144 bytes is the hard ceiling list_workbench_documents/list_project_documents enforce. 200,000 leaves 62,144 bytes (24%) of headroom so a part that crosses the threshold mid-append still has room to finish the entry in progress before the next append has to roll to a new part.
---
Rotation threshold is 200,000 bytes per part.