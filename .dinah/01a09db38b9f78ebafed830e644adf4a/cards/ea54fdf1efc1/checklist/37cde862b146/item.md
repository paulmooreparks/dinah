---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:10Z
ordinal: 21
note: "`SetWorkbench` and `SetWorkstream` both refuse `not-operator` for an actor who is not the operator; `SetCardField` requires an owner and accepts any, because a classification is not a claim. Nothing writes a column field today, so the column's rule is being minted here, and it is minted with its two workbench-level siblings: a column is a station of the flow, `reshape` rewrites it from a definition, and letting any owner rename a station or change its kind puts the flow in the hands of whoever happens to hold a card. Declaring the authority per kind rather than branching inside the writer is what lets a test assert that no kind slipped in with no rule, which is AC-8."
---
A write to a column's fields is the operator's, which mints a rule rather than moving one, and `bench.WriteAuthorityOf` declares the authority per kind from a closed pair.