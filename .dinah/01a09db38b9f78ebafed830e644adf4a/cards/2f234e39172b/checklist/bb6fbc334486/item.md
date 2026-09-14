---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:37Z
ordinal: 24
note: "Yes, via one check added to the shared closeItem (which Resolve, Verify and Fail all call): an item whose stored owner is exactly \"operator\" refuses a non-operator's close with the existing not-operator refusal. One check covers all three terminal verbs and all three item kinds, since \"is this item the operator's\" is a question about the item rather than about which verb is landing it."
---
Is settling (resolve/verify/fail) an owner="operator" item refused to a non-operator, and where is that enforced?