---
kind: acceptance_criterion
state: pending
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:20Z
ordinal: 1
---
A test in internal/verb exercises claim(actor A) -> release -> claim(actor A) in immediate succession and asserts both claims succeed; go test ./internal/verb/... stays green after this card's implementation lands.