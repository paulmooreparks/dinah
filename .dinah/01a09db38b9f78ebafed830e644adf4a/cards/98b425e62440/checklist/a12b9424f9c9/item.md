---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
owner: holder
ts: 2026-09-14T02:16:35Z
ordinal: 10
note: "Arming: removed \"format\": true from sessionFlagNames map and reran the test. The \"format\" subtest reddened by name with \"format is no longer a session flag, so this case is asserting nothing\" while every other subtest stayed green. Restored clean and reconfirmed green."
---
TestParseArgsRecordsNoDomainCaptureForASessionFlag asserts its roster and sessionFlagNames have the same membership, in both directions. The per-name loop covers roster-to-map, where the `if !sessionFlagNames[name]` fatal, whose message reads "is no longer a session flag, so this case is asserting nothing", fires when a name leaves the map. The map-to-roster direction is the loop after it: every key of sessionFlagNames that has no case in the roster reaches the t.Errorf whose message reads "is a session flag with no case in this guard's roster", naming the flag and saying that nothing holds it out of domainCaptures. Verified by adding `"format": true` to sessionFlagNames without touching the roster and watching this test go red naming `format`, then removing it and watching it go green. Before this repair that edit left the test passing, and it is the edit PR #123 (dinah-31) makes, on a line this branch does not touch, so the two merge cleanly with no conflict marker. Whichever of the two pull requests lands second adds `"format"` to the roster in this test, and the red the second landing produces goes away.