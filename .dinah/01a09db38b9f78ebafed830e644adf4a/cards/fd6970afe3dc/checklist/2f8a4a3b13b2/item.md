---
kind: acceptance_criterion
state: pending
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:19Z
ordinal: 6
---
Every journal field or event member the implementation actually adds is named in docs/design/format.md's moved-event schema row, and wantedEvents[contract.EventMoved] in cmd/dinah/compat_test.go includes that member's name. TestTheSampleFixtureCarriesEveryShapeThisBuildWrites and TestReplayingThePopulationSequenceReachesEveryShapeItNames both pass after the change, which requires recapturing the sample fixture (with the repository's own capture script) when the new member is written by a fresh replay but absent from the frozen sample. A run of either test failing, or a run that passes only because the new member was never exercised by populate.txt, fails this criterion.