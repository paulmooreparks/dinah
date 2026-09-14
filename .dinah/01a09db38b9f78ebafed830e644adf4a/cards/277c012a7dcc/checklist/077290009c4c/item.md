---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:20Z
ordinal: 17
note: "`references_command_resolution_test.go:parseReferencesGuideTable`'s own comment says the guide is read as the declaration because it is \"the one place the tool already declares which kinds a command takes\". That was a stopgap. `internal/verb/reference_kinds.go` becomes that one place, the guide's table is checked against it cell for cell, its workstream sentence is checked against it by section 5.8, and both heads render from it. The guide stays derived, which is the relation dinah-457 already established for its roster. Round 2 narrowed what the shipped doc comment on `referenceKinds` may claim. Round 1 opened it with \"the one statement of which command takes what\", and the resolver contradicts that: the resolver decides what a command accepts and the map only declares it. This board has found a \"single declaration\" claim false three times by tracing rather than reading, so section 2 states the true, narrower claim, that the map is where the three published answers come from, and says outright that the resolver decides. Round 3 restates the coverage figure the comment points at: with the sixth kind the declaration is eighteen commands by six kinds, and the probes hold fifty-two of those one hundred and eight cells against the running binary, where before the card it was thirty-four of ninety."
---
The declaration moves out of the guide's Markdown table and into `internal/verb`, and the table is held to it in both directions.