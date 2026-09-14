---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:30Z
ordinal: 10
note: "Re-run: go test ./internal/verb/... -run TestEveryDeclaredGuardIsRouted and go test ./internal/bench/... -run 'TestEveryKindDeclaresItsFieldsAndItsAuthority|TestAllFieldsIsTheSortedUnionOfEveryKind' all pass."
---
TestEveryDeclaredGuardIsRouted (internal/verb/fields_routing_test.go) passes with GuardHold added to bench.Guards and routed in internal/verb/fields.go; TestEveryKindDeclaresItsFieldsAndItsAuthority and TestAllFieldsIsTheSortedUnionOfEveryKind (internal/bench/fields_test.go) pass unmodified against the new field.