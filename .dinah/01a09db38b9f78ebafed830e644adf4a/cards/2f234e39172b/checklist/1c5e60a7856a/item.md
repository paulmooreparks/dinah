---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:37Z
ordinal: 25
note: Reopen stays open to any owner. Reopening returns a closed item to pending, which ItemLiftsColumnHold already treats as not-lifting, so reopening can only re-impose a hold, never lift one; restricting it would protect nothing the operator-ownership guarantee needs protected. Fail, unlike Reopen, is a settling verb exactly like Resolve and Verify (all three reach closeItem) and is restricted identically to them.
---
May a non-operator reopen or otherwise touch an owner="operator" item, given that closing one is now refused?