---
kind: acceptance_criterion
state: failed
ts: 2026-09-24T03:23:30Z
ordinal: 1
citations:
  - scheme: test
    target: internal/verb/mutate_test.go#TestTheCardLockCoversTheWholeTransaction
    observed:
      before: fail
      after: pass
resolution: 8a0ae392772e
---
the endpoint returns 404 for an unknown id