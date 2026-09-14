---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:07Z
ordinal: 18
note: "The check has to name the workstream kind, and naming it as a literal would make an eighth spelling of a word the tree already spells seven times. The set was produced by searching the tree rather than by reading the files already open: internal/bench/entity.go:383 assigns it, internal/verb/beyond.go compares it at 639, 652, 697, 710, 736 and 748, and beyond.go:190 spells the attachment kind the same way. internal/bench/kindguard_test.go scans for column-kind literals and would have to grow a second vocabulary to cover entity kinds, which is a card of its own rather than a line of this one, so AC-9 runs the search at Test and says plainly what it does not prove."
---
`bench.KindWorkstream` is minted and the seven entity-kind literals are converted, and the guard for it is a search rather than a test.