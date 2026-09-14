---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:17:13Z
ordinal: 5
note: "go test ./internal/verb/... -run TestABlockedCardAtAQueueColumnStillDrawsItsGroup green. Manual fixture reproduces: blocked a card standing at Queue (dinah.buffer) and separately at Outside (awaiting_outside:true); both drew their own blocked group via the carried-value path with no card lost from the tree's total count."
---
A card actually standing blocked at a column that declares no state at all still draws its own group there, rather than being silently dropped from a tree whose root count still includes it. Test hook: go test ./internal/verb/... -run TestABlockedCardAtAQueueColumnStillDrawsItsGroup, new test specified in the spec.