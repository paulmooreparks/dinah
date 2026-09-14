---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:25Z
ordinal: 19
note: "Established by running: `dinah join wb-1 addressing` and `dinah join wb-1 65cd4a46c729` both exit 0 against a binary built from 46abf67, while `dinah path addressing` and `dinah path 65cd4a46c729` both raise unknown-card. `dinah help join` already documents the bare form as \"the workstream you are naming, written as its slug or its identifier\". Agent Design Review offered narrowing as the second answer, and this declines it: narrowing changes join, leave, get, set and the query filter, breaks scripted calls that work today, and makes a shipped help page false, against one sentence for teaching. The asymmetry itself is designed rather than accidental, because orAWorkstreamNamedBarely's own comment records that a bare handle is read as a card where a reference is read as an address so a workstream cannot shadow a card, so the sentence has to say where the bare form is refused. These are two more forms the tool accepts and the guide never teaches, which is this card's subject rather than a neighbouring one, so they join the roster and workstream-bare-slug leaves the near-miss roster as an accepted form while the same string stays there as a near miss keyed to dinah path."
---
The bare workstream slug and the bare workstream identifier are taught in the guide rather than narrowed out of the tool, and the guide states where the bare form is refused.