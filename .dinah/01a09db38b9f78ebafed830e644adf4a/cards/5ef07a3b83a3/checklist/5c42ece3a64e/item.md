---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:17:44Z
ordinal: 17
note: Applying dinah-484's rule naively would move criteria onto Test with an exit hold, since Test is the station that settles them, and that would have the same effect one column earlier. It is not done, because the operator's preserved ruling is that Merge keeps holding on the way in and a criterion still has to be settled before the card arrives there. Test still gets `hold out`, for the decisions a tester takes in the course of the work, which name Test and are settled there. An item names exactly one column, so a criterion naming Merge and a decision naming Test never meet and neither weakens the other. Recorded because a reviewer applying the new rule mechanically would read the Test row as an inconsistency rather than as the operator's ruling being honoured.
---
Acceptance criteria keep naming Merge and are held at its entry, while Test holds on the way out for its own items. The two mechanisms run side by side rather than one replacing the other.