---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:27Z
ordinal: 1
note: "Re-verified at Test on merged head 9f39d11 (branch already carries current origin/main f574ac5). go test ./internal/verb/ -run \"TestEveryDeclaredGuardIsRouted|TestEveryGuardIsDeclaredByAField\" -v: both PASS."
---
bench.FieldOf("item", "column") reports a field whose Guard is bench.GuardColumnRef; GuardColumnRef is a member of the closed bench.Guards list; TestEveryDeclaredGuardIsRouted passes with a case bench.GuardColumnRef: arm routed in admitFieldValue.