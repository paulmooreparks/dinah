---
kind: decision
state: resolved
ts: 2026-09-14T02:17:01Z
ordinal: 21
note: No static check over msg.For call sites can see a code path that never calls msg.For at all -- that is a cmd/dinah refusal-rendering defect, not a catalog or translation-guard concern, and belongs to whoever fixes that rendering code.
---
The reachability guard covers only the msg.For-pinned-to-one-language pattern; the CLI's bare-refusal-name rendering bug from dinah-245 is out of this card's scope.