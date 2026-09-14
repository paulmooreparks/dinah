---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:37Z
ordinal: 28
note: Name the column that settles an item as an exit-hold target on that same column (replaces the old downstream-entry-hold workaround for review-station questions, and cannot self-deadlock the way naming your own column as an entry-hold target does). Name a column that depends on an item already being settled as an entry-hold target on itself, unchanged (the genuine "prove this before that" case, e.g. an acceptance criterion gating Merge). Recorded forward for the workbench instructions text and the dinah-449 cutover, since this card cannot edit that text.
---
Which direction should each kind of item use once both exist, given that the board's current guidance ("gate the column after the one that answers it") is a workaround for entry being the only direction?