---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
owner: holder
ts: 2026-09-14T02:17:54Z
ordinal: 1
note: "VERIFIED. internal/verb/collection_roster_test.go TestTheReferenceTakingRosterIsFifteen. The roster is derived at this commit by verb.ReferenceTakingCommands(), which reads the two declarations the spec's script parses (the guides table and each command's own Guide field) and returns a command only when both agree. Run: `go test ./internal/verb/ -run TestTheReferenceTakingRosterIsFifteen -v`. It logs \"the roster derived at this commit holds 15 commands\" and the set equals the fifteen the criterion names. The two declarations are also counted separately, both fifteen, so a disagreement is reported rather than shortening the list silently. DEVIATION FROM THE SPEC, and it is stronger rather than weaker: the roster is read off the tables themselves rather than regexed out of the source text, so a formatting change to definition.go cannot make the parse find nothing. ARMED: removed `Guide: \"references\"` from archive's parameter; the plant compiled and ran, and the assertion at collection_roster_test.go:21 reddened with \"the roster holds 14 commands and it is fifteen\"; restored byte-identically, green."
---
The roster of commands taking a reference is derived at the commit under test rather than read off this card, and it is fifteen. Running spec section 2.1's script over internal/verb/definition.go prints two rosters that agree, each of length 15, and the set equals {archive, attach, attachments, cite, contents, delete, edit, fail, instructions, path, rename, reopen, resolve, show, verify}. The run records the number it found beside the verdict.