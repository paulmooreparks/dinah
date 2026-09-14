---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:50Z
ordinal: 42
note: "One title makes one card, so fanning out would file the same title into several columns, and acting on the invoked row alone is the quiet narrowing the operator ruled against. Refusing is neither: the reader is told why and nothing happens. The message is `dialog.bulk.oneRowOnly`. This is the only command in the table carrying the `rowOnly` policy, and the policy exists so that a later command with the same shape has somewhere to declare itself."
---
New Card refuses a selection of more than one column rather than acting on the row it was invoked on.