---
kind: decision
state: resolved
ts: 2026-09-14T02:18:05Z
ordinal: 25
note: "TestEveryShippedGuideFitsEveryWindowItIsWrappedFor exempts a table row by name at guide_wrap_test.go:275, so it is the one test that must not measure a table row, and TestAGuideTableSurvivesTheWindowItIsReadIn asserts four row literals at 40 columns and measures nothing. Nothing in the suite held the table to eighty, so the header rename D-3 pays for was guarded by nothing and the next person to lengthen a heading would fold every row with the suite green. TestTheReferencesGuideTableFitsAnEightyColumnWindow, spec section 5.7, measures every line of the table with displayWidth (cmd/dinah/row.go:59) and checks the line count against the roster plus two, so a table that lost rows fails on the count. Both plants were run at b825059 against the draft: the old header draws 83 and fails, a deleted row measures 16 against a wanted 17 and fails. This makes six new tests rather than five."
---
The eighty-column table width gets its own test rather than an assertion added to the wrap test or to the roster test.