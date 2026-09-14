---
title: Three conformance statements are excused as out of reach, and all three are things the tool now does
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
workstreams:
  - f1fd8d672caf
---
`internal/profile/conformance_test.go` carries a table of statements the v0 surface is excused from driving, each with a sentence saying why it is out of reach. Three entries in that table are about a card's structured checklist item, and all three assert something the tool stopped being true of some time ago:

- `CORE-ITEM-1`: "a permission, and v0 stores no structured item on a card, so there is nothing for a card to be offered carrying"
- `CORE-ITEM-2`: "the tool reports no structured item, so it is never asked whether one is resolved"
- `CORE-ITEM-3`: "no verb consults a structured item, so no refusal can follow from one and no refusal name can be reported for one"

The tool stores a structured item, reports one, and consults one at the move gate. The third statement is the exact subject of dinah-450, which gave a checklist item's column real teeth at move time, and of dinah-474, which made both write paths agree about what that column may hold. So the table now excuses the suite from proving the very behaviour two shipped cards were about.

This is worse than three stale comments, because the table is what stands between a statement and the requirement that a test drive it. An entry here does not merely describe the code, it suppresses a check. A false entry means the conformance report reads clean while nothing at all pins the behaviour.

Why it was not cleared where it was found. It surfaced during dinah-474's sweep of false claims, and that card corrected fourteen of them in place. These three could not go the same way, because the fix is not a comment edit: removing an entry from this table obliges the suite to name a test driving that statement, so closing all three means writing three conformance tests. The reviewer on dinah-474 ruled them onto one consolidated card for that reason.

One thing worth carrying into whoever works this. Two of the three are worded so that no phrase search reaches them; they were found only by reading around a line a search did return. So a sweep for the same class elsewhere in this table cannot be a grep, and a search coming back empty here is not evidence of absence.

Read dinah-474 and dinah-450 before starting, and check whether any other entry in the same table has gone false the same way rather than fixing only the three named here.
