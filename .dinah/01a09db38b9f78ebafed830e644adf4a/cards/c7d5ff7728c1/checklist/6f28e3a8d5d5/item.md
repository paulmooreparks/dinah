---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:01Z
ordinal: 28
note: The ten-against-fifteen drift is the reason. The sentence opening the guide's "Which command takes what" section counts ten commands and the code points fifteen at that guide, and nothing noticed, because a hand-written list is only as good as whoever last edited it. The guide's own opening sentence is "You name a thing to Dinah by writing a reference."; the ten-command claim belongs to that section rather than to the guide. A criterion that checks a hand-written list proves that somebody maintained the list. The test derives the roster from the `guides` map in internal/verb/definition.go and fails on a command in the map with no row and on a row naming a command the map does not carry, so it fires in both directions.
---
The references guide's command table is generated from the code rather than kept in step by hand, and a test fails when the two sets differ.