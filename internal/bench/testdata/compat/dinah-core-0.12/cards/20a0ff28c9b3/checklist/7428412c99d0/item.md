---
kind: acceptance_criterion
state: failed
ts: 2026-09-07T19:07:50Z
ordinal: 1
citations:
  - scheme: test
    target: internal/verb/mutate_test.go#TestTheCardLockCoversTheWholeTransaction
    observed:
      before: fail
      after: pass
note: the handler still answers 200 for an unknown id
---
the endpoint returns 404 for an unknown id