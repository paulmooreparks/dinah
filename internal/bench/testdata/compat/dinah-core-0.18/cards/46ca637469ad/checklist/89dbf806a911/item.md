---
kind: acceptance_criterion
state: failed
ts: 2026-09-20T03:28:35Z
ordinal: 1
citations:
  - scheme: test
    target: internal/verb/mutate_test.go#TestTheCardLockCoversTheWholeTransaction
    observed:
      before: fail
      after: pass
resolution: sample-1/criteria/1/comments/2
---
the endpoint returns 404 for an unknown id