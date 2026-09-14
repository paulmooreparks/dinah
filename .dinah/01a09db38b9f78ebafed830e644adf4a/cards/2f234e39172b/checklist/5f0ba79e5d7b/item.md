---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:37Z
ordinal: 30
note: "Immediately after the loop-limit block and before three checks that already stand between it and canLand's return: the TakesNoWork/AwaitingOutside row, the OperatorReservesClaim row, and the retiring row. None of those three carries its own entry in checkLists[Move], so they are unprinted for move, but they still execute and could otherwise win the race to return ahead of a printed check running after them. The prior revision placed the new block after those three (immediately before canLand's closing return), which Agent Design Review found contradicts row 11's own claimed position right after row 10; this decision corrects the runtime position to match."
---
Where inside canLand does the departure's exit-hold check run, given AtLoopLimit is not canLand's last check on trunk?