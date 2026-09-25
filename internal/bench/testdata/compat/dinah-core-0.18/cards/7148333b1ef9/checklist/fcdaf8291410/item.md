---
kind: acceptance_criterion
state: failed
ts: 2026-09-25T07:08:38Z
ordinal: 1
citations:
  - scheme: test
    target: internal/verb/mutate_test.go#TestTheCardLockCoversTheWholeTransaction
    observed:
      before: fail
      after: pass
resolution: 0327e543af99
---
the endpoint returns 404 for an unknown id