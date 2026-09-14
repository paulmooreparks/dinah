---
kind: decision
state: resolved
ts: 2026-09-14T02:18:41Z
ordinal: 20
note: Instructions (internal/verb/read.go:1306) replaces any resolution error with dinah.unknown-path and CardAffordances (internal/verb/library.go:681) answers the workbench-level affordances when the reference does not resolve. Round 2 named that second function Affordances; the function at that position is CardAffordances, and ServedAffordances immediately below it at line 703 is a different function that flattens the same way. Both are deliberate and carry comments saying so, both already flatten today's unknown-card identically, and neither answers with a card the caller did not name, so neither carries this card's defect. Changing either is a decision about that verb's own refusal vocabulary rather than about card resolution, and this card enforces a rule at the resolver rather than rewriting what each verb does with a refusal. They are named in the spec because a harmless instance of the shape is a finding and silence about it is not.
---
The two callers that flatten a resolver error, Instructions and CardAffordances, are left alone and named rather than changed.