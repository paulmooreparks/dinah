---
kind: decision
state: resolved
owner: operator
ts: 2026-09-14T02:17:29Z
ordinal: 32
note: "Paul ruled on 2026-09-11, at Operator Design Review, on the full trade as the spec states it: the API does not exist below 1.101, so anyone on an older editor stops receiving every future update to the extension rather than only this feature, and nothing tells them it has stopped. He took the plain version of that rather than the two alternatives put to him, which were announcing the floor in the release notes and README, and shipping two builds so nobody is stranded. The two-build option is argued against in the spec, and this ruling does not reopen it. No prose obligation follows from this ruling: it is a decision to accept the cost, not to publicise it, and a later card wanting to announce the floor is a separate piece of work."
---
The extension's minimum VS Code version rises to 1.101, and the feature ships.