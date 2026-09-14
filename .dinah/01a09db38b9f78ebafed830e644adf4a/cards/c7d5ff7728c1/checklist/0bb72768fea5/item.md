---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:02Z
ordinal: 36
note: "Found by sweeping for the reported defect's shape rather than repairing only the place it was reported. The round-3 AC-4 required all six child cards to sit in Intake, while spec section 9's own bullet 4 records that dinah-459 has landed on the trunk at 813e0bb. Read against the board on 2026-09-09 with list_cards_brief: dinah-459 is in Acceptance, dinah-454 is in Implement and active, and dinah-455, dinah-457, dinah-460 and dinah-461 are in Intake. So the criterion already failed on two of six, and it failed because the workstream is being worked. AC-4 gates Merge, so the failure would have been read at Test, by which time the position it asserted had been untrue for weeks. The three properties kept are the ones a filing can get wrong and that stay true once right. The criterion states in its own text what dropping the column check costs, which is any proof that the cards were filed into Intake rather than created somewhere else."
---
AC-4 checks that the six child cards exist, are joined to the addressing workstream, and name the spec section that specifies them, and it checks nothing about which column each stands in.