---
kind: acceptance_criterion
state: failed
ts: 2026-09-25T06:55:33Z
ordinal: 1
citations:
  - scheme: test
    target: internal/verb/mutate_test.go#TestTheCardLockCoversTheWholeTransaction
    observed:
      before: fail
      after: pass
resolution: 311f39788337
---
the endpoint returns 404 for an unknown id