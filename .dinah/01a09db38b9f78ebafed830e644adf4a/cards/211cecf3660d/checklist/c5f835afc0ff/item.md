---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:27Z
ordinal: 5
note: "Re-verified at Test end-to-end, the decisive proof. `dinah set doing hold on`; filed a bare item on a fresh card wb1-2 with `dinah file wb1-2 acceptance_criterion \"must resolve first\"` (no --column); `dinah set wb1-2/criteria/1 column doing` (slug, generic write path) exited 0 and stored 15a9235a6266 (Doing's id). `dinah move wb1-2 doing` was then refused: \"unresolved-item this card carries the item c48155c92fc9, which is not resolved\". `dinah verify wb1-2/criteria/1 \"resolved for AC-5\"` then `dinah move wb1-2 doing` exited 0, landing the card at [Doing / ready]. A column written through the generic write path by its short name actually held the card at move time."
---
End-to-end proof that a validated write actually holds a card: create a column with hold on, file an item naming that column by its slug through dinah set <item> column <slug> (not by identifier), attempt dinah move <card> <that column> and confirm it is refused unresolved-item; resolve the item and confirm the same move then succeeds.