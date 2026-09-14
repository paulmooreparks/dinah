---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:26Z
ordinal: 22
note: "Round 1 put the roster in internal/bench with Kind as a plain string, because internal/bench imports internal/verb nowhere and a cycle would be needed to use the real type. A separate package removes that constraint: internal/addressform imports internal/verb, nothing in verb or bench imports addressform, so Kind is verb.ReferenceKind rather than a string. Putting the guards in cmd/dinah rather than in internal/bench avoids the other cycle, since a test file in package bench importing addressform would import bench through verb; cmd/dinah already imports both, already carries repositoryRoot and testFunctionsUnder, and cmd/dinah/address_sweep_test.go is the precedent for an AST guard about another package living there. internal/addressform being read by tests alone is the shape internal/guide/guidepin already has, and it exists for the same reason, which is that a declaration two test packages both read has to live where both can reach it."
---
The three rosters live in a new package `internal/addressform`, and every guard reading them lives in `cmd/dinah`.