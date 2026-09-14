---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:40Z
ordinal: 1
note: TestAUniqueNumberResolvesExactlyAsBefore (internal/bench) drives resolveCardIn against both roots; TestAUniqueCardNumberResolvesOnTheHeadAsBefore (cmd/dinah) drives show and path, and fx-404 still refuses unknown-card.
---
Compatibility, driven rather than asserted: against a fixture holding one card with number 1, `dinah show fx-1` and `dinah path fx-1` resolve to that card exactly as on the parent commit, and `dinah show fx-404` refuses `unknown-card` exactly as on the parent commit. A Go test in internal/bench asserts the same two cases directly against resolveCardIn for both the live root and the archived root. Only the two-or-more-matches case changes behaviour.