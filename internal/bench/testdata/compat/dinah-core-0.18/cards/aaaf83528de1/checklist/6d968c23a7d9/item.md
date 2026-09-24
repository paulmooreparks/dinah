---
kind: acceptance_criterion
state: failed
ts: 2026-09-24T06:33:17Z
ordinal: 1
citations:
  - scheme: test
    target: internal/verb/mutate_test.go#TestTheCardLockCoversTheWholeTransaction
    observed:
      before: fail
      after: pass
resolution: ee2e769e2d34
---
the endpoint returns 404 for an unknown id