---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:18Z
ordinal: 19
note: "Round 1's ResolveEditTarget asked ResolveReference first, and that resolver answers an empty reference with the workbench, so `dinah edit` with no argument would have opened workbench.md. That contradicted AC-5, which requires the empty reference to keep refusing. The criteria are kept and the design gains arm 1, which sends a reference empty after TrimSpace straight to ResolvePath. Three things back that direction and none backs the other: IsWorkbenchRef's doc comment at internal/bench/bench.go:2001 already declares the rule and names `edit` while declaring it, so opening the workbench would falsify a comment about this very command; internal/verb/definition.go:565 declares edit's parameter Required, and since runEdit reads at(parsed.rest(), 0) and gets the empty string from an absent argument, the resolver's refusal is the only thing enforcing that declaration; and D-3 refused to move `path`'s settled answer to fix `edit`, so letting a bare `dinah edit` succeed while a bare `dinah path` refuses splits the two commands the other way for no reason. The refusal is delegated to ResolvePath rather than restated with contract.Refuse, so the sentence a reader gets cannot drift from the one `path` raises. Observed at 808d105: `dinah edit` with no argument, with \"\", with three spaces and with a tab all print `unknown-card this workbench carries no card ;` and exit 2, while `dinah get \"\" title` exits 0 and answers the workbench."
---
The empty reference keeps refusing; the design is corrected rather than the criteria.