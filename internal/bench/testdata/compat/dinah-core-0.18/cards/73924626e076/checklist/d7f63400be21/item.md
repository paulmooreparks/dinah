---
kind: acceptance_criterion
state: failed
ts: 2026-09-23T07:45:47Z
ordinal: 1
citations:
  - scheme: test
    target: internal/verb/mutate_test.go#TestTheCardLockCoversTheWholeTransaction
    observed:
      before: fail
      after: pass
resolution: 82bc384844e6
---
the endpoint returns 404 for an unknown id