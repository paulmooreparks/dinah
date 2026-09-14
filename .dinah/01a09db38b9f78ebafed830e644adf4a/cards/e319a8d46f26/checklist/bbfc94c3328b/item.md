---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:51Z
ordinal: 49
note: "Round 1 put three counts on BulkReport and left it to every caller to keep them closing over the selection. That is the shape this board keeps finding: a rule that has to reach every caller, correct in every caller anybody checked. It also had a live instance, since the clean-run message was filled from `selected` rather than from what succeeded, so a run that lost two rows would have said \"All 5 selected rows are done\" while two cards were still on the board. `runOverRows` removes the possibility rather than the instance: `selected` is `rows.length`, one entry is appended per row before the loop advances, and a caller cannot supply either number. `summaryFor` then refuses a report that does not add up, which catches any future path that builds a report literal by hand. The counterexample \"An accounting whose stated total covers an item its own enumeration never names\" is this defect one level up, and its rule is that an accounting is checked by matching its parts to its members rather than by adding it up, which is what the per-index assertions in AC-20 do."
---
The loop lives in `bulk.ts` and builds the report, rather than each command accumulating its own counts.