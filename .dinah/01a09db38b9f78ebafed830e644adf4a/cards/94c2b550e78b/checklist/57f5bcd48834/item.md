---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:32Z
ordinal: 12
note: "Verified at 5a88bec, both halves armed, and the arming found a defect that was fixed on this card. Provenance half: renaming TestAPositionUnderTheFlagCountsTheMirrorsOwnMembers to ...Renamed reddened the sweep with \"the pin on the references guide names TestAPositionUnderTheFlagCountsTheMirrorsOwnMembers as the test proving it and no test function of that name stands under <root>\". File-count half: pointing the scan at an empty directory reddened it with \"no _test.go file stands under <root>, so the provenance half of this sweep read nothing\". The first run of that second half exposed the defect: the message named repositoryRoot while the scan had read elsewhere, sending a reader to the wrong tree. The root is now held in a `provenanceRoot` variable used by both the call and every message, and the re-armed run named the empty directory correctly. The sweep logs both counts: \"5 pinned statements checked, 171 _test.go files scanned\". Restored byte-identically and green."
---
TestEveryPinnedStatementStandsInItsGuideAndNamesALiveTest fails when a statement's Provenance names no test function in the tree, and it fails when its scan for test functions read no _test.go file at all. It logs both the number of statements checked and the number of test files scanned. Armed by renaming one proving test function and by pointing the scan at an empty directory.