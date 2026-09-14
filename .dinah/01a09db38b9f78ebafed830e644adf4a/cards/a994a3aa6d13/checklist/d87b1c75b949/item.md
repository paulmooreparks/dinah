---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:48Z
ordinal: 8
note: "PASS, and the criterion genuinely had the chance to fail. Built as the criterion prescribes, with sibling workbenches `plans` and `plans-archive` so that walk order and path order diverge. The directory walk yields `plans` first; the answer came back `plans-archive` first, which is the sorted order and the opposite of the walk order. Both rows carry title and path. Noted by the verifier: the divergence only exists because `path` is the .dinah/<id> directory; had it been the sibling directory itself, `plans` would be a prefix of `plans-archive` and the two orders would coincide, making the criterion untestable. It is testable as built."
---
Against `dinah mcp --root <dir>` where two workbenches lie under `<dir>`, a `tools/call` of `workbenches` answers with both rows, each carrying `title` and `path`, and the rows are in ascending order by their `path` member, equal to a sorted copy of the same rows.