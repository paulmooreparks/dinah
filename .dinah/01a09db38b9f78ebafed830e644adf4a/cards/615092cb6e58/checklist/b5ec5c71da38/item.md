---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:15Z
ordinal: 17
note: "A column's position lives only in the workbench anchor's columns sequence, and archiving removes it there through RemoveColumnID, so the position is already gone when a restore runs. Recording it on the column anchor adds a format key meaning nothing while the column is live, recovering it from the journal makes the order depend on replay, and refusing to restore columns would make restore an inverse for five kinds out of six. Appending is one line, and dinah reshape already moves a column. The occupancy scan is separate and is the one behavioural change to Bench.Run: it refuses dinah.occupied when a live card names the column, which is right for an archive and backwards for a restore, because a live card naming an unlisted column is the stranded state dinah check reports and restoring the column is the repair. Refusing the repair because the damage exists is a check that fires against correct code."
---
A restored column is appended to the end of the workbench anchor's columns sequence, and Bench.Run's occupancy scan is skipped on a restore.