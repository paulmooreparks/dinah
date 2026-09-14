---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:37Z
ordinal: 27
note: "Nothing new. `Column.Hold`'s four values, `OperatorOwned` and `AwaitingOutside` combine exactly as the prior revision argued for two independent booleans, now over one field: `OperatorOwned` alone is the acceptance-station shape the operator ruled must be preserved (nobody else moves a card out, regardless of `Hold`, because the non-operator departure check in `canRoute` runs before `canLand`'s exit-hold check ever does); `AwaitingOutside` combined with `Hold` carrying `out` or `both` is the review-station shape this card exists to enable; an unused `out`/`both` declaration is exactly as inert and exactly as legitimate as an unused `on` declaration is today. The one thing that would make a declaration meaningless, an item naming a column the workbench does not declare, is already refused at write time by dinah-473/474."
---
What should `dinah check` report about a workbench whose hold declarations no longer make sense?