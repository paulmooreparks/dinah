---
kind: decision
state: resolved
ts: 2026-09-14T02:18:04Z
ordinal: 15
note: A table row is exempt from the guide wrap and reproduced whole, so the terminal folds a row the wrap will not. The existing table is 68 columns; adding "A collection" without the rename makes it 83 and folds every row at 80. The rename brings it to exactly 80 and makes the six headers one register. The cost is two literals in cmd/dinah/references_command_resolution_test.go, at :116 and :131, which the spec names.
---
The table's first column header changes from "This workbench" to "A workbench", and the new collection column takes the room that frees.