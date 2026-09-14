---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:32Z
ordinal: 16
note: Verified by internal/verb/link_test.go:TestLinkRefusesItsPreconditionsInTheProfilesOrder, which runs both verbs and both refusals. The no-operator case names a card that would otherwise be admitted, so it proves precedence rather than coincidence; the no-owner case passes an empty kind as well and still reports no-owner, so it proves the owner check runs ahead of the argument checks.
---
`link` and `unlink` each refuse `no-operator` when the workbench designates none, and `no-owner` when the request carries no actor, before any other check runs, matching every other write verb's ordering.