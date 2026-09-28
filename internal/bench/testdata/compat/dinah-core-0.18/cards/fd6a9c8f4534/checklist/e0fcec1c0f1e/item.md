---
kind: acceptance_criterion
state: failed
ts: 2026-09-28T23:01:26Z
ordinal: 1
citations:
  - scheme: test
    target: internal/verb/mutate_test.go#TestTheCardLockCoversTheWholeTransaction
    observed:
      before: fail
      after: pass
resolution: 497b09c7bbd6
---
the endpoint returns 404 for an unknown id