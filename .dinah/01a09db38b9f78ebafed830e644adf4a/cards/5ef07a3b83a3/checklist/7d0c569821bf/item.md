---
kind: acceptance_criterion
state: pending
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:43Z
ordinal: 4
---
Exactly one column is owned by the operator outright, and it is Acceptance. The parsed `dinah export` output is swept for `operator_owned`: exactly one of the 14 elements carries it, that element's slug is `acceptance`, and the run prints both the count it found and the total it swept, so a sweep over an empty array reports zero rather than passing. Neither `operator-design-review` nor `operator-code-review` carries it, and both carry `gate_items` set to `"out"`. Then the thing that makes this shape work rather than merely permissive is proven by provoking it on a scratch copy, because a hold that never refuses the operator would leave his stations with no stop at all: at each of those two stations, with the card carrying a pending item owned by the operator and naming that station, THE OPERATOR'S OWN move out is refused `dinah.unresolved-item-exit`, and with the item settled the same move succeeds. A failing run is a count other than one, the wrong slug, either review station still owned, either missing `gate_items`, or a refusal that does not fire against the operator himself.