---
kind: acceptance_criterion
state: failed
ts: 2026-09-24T08:09:45Z
ordinal: 1
citations:
  - scheme: test
    target: internal/verb/mutate_test.go#TestTheCardLockCoversTheWholeTransaction
    observed:
      before: fail
      after: pass
resolution: c54ba0fcdd2c
---
the endpoint returns 404 for an unknown id