---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:25Z
ordinal: 16
note: "Superseded in part by D-13 at round 2, which moves the roster out of internal/bench into a new package internal/addressform; the pin mechanism this decision settles is unchanged and stands.\n\nPin mechanism: guidepin.Pinned's own test asserts len == 5 with the reason that the archive section makes five claims, so pouring this card's address sentences into it would make that number mean two things at once. Carries is exported, takes a topic and a sentence, and folds both sides with strings.Fields before searching, so the address roster uses it directly and guidepin needs no edit. AC-10 holds Pinned at five and holds its own test unedited.\n\nThe home half of the round-1 decision is retired. It reasoned that a roster in internal/verb could not be read by the AST guard's subject or by pick, and that internal/bench was therefore the only home; round 2's guards live in cmd/dinah and read the roster from a third package, so neither constraint binds. See D-13."
---
The roster's guide sentences are held through `guidepin.Carries` rather than by extending `guidepin.Pinned()`.