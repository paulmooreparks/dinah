---
kind: decision
state: resolved
ts: 2026-09-14T02:17:01Z
ordinal: 15
note: "Composing the token set from live declarations (substates, commands, flags, level axes) needs internal/verb, internal/bench and internal/contract, and internal/verb/read.go already imports internal/msg, so msg importing verb back would cycle. internal/verb already has precedent for this shape: TestVersionCarriesTheConformanceClaim reads msg.Complete and msg.Skeleton from outside msg."
---
The contract-token guard lives in `internal/verb`, not `internal/msg`.