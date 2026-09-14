---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:10Z
ordinal: 19
note: "`workstream get autumn status` accepted the bare slug because the command took a workstream and could have meant nothing else. `Bench.ResolveReference` tries a bare head against the columns and then the cards, and a workstream names its own kind in the grammar precisely so a bare handle cannot be shadowed by a column or a card sharing its name, which `Workstream.Ref` states at internal/bench/workstream.go:93. Accepting the bare slug on `get` would mean a column called `autumn` and a workstream called `autumn` resolving differently depending on which command you typed, which is the disease this workstream exists to cure. The quick start's sentence about both spellings is rewritten rather than deleted: `join` and `leave` still take a workstream and nothing else and still accept either spelling. Spec section 9.2 names the guard that hard-codes the old invocation and the line it becomes."
---
A workstream reference given to `get` or `set` carries its `workstream/` prefix, so the bare slug the retired pair accepted stops working on the generic one.