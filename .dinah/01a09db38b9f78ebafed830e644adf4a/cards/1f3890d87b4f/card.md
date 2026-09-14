---
title: The Implement column's self-test does not run every check CI runs
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
The Implement column's `self_test` directive names `go build ./...`, `go vet ./...` and `go test ./...`. The GitHub workflow runs those three and a gofmt job. So a formatting-only defect passes every check the column asks an implementer to run and fails the first check the pipeline runs.

That happened on dinah-193 on 2026-08-23. A new constant landed beside an existing one without the alignment gofmt gives a const block. Build, vet and the full suite were green locally, the card moved through code review and test, a pull request was opened, and gofmt failed there. The repair was one commit changing whitespace, but the round trip cost a CI cycle and an operator interruption for a defect the implementer could have caught in one second.

The fix is to add `gofmt -l .` to the column's `self_test`, and more generally to keep the directive and the workflow in step: a check the pipeline enforces and the directive omits is a check nobody runs until it is expensive to fail. Worth deciding whether the two should be derived from one source rather than maintained separately, since they will drift again otherwise.
