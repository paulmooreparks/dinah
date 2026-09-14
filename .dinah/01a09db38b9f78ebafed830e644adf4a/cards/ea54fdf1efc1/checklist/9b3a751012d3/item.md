---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:09Z
ordinal: 8
note: "The authority arm is what stops a kind slipping in with no rule about who may write it, which would default to whatever the router happens to do. Plant that reddens the first: return the empty string from `WriteAuthorityOf` for `column`. It compiles and the run reports that column declares no write authority. Plant that reddens the second: add a `GuardOrdinal` constant and use it on a field without writing an arm; the run reports a declared guard nothing routes. Neither check recomputes its expectation by calling the code under test, because both compare two independent declarations, the field table and the router's own switch."
---
Every kind declares a write authority from the closed set, every declared guard has a routing arm, and `bench.FieldsOf` answers for every kind the grammar names. `internal/bench/fields_test.go`, `TestEveryKindDeclaresItsFieldsAndItsAuthority`: `bench.EntityKinds()` returns the six kinds `containment.go` names plus `workstream`, every one of them reports a non-empty `FieldsOf`, every one reports a `WriteAuthorityOf` that is `AuthorityOwner` or `AuthorityOperator`, and a kind name the grammar does not carry reports no fields and is not in the list. `internal/verb/fields_routing_test.go`, `TestEveryDeclaredGuardIsRouted`: every guard constant a field declares has an arm in the write router, the arm set carries no name the constants do not declare, and the run is fatal when no guard is declared at all.