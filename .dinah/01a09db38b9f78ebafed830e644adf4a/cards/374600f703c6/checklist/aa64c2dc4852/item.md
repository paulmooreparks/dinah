---
kind: open_question
state: resolved
column: c9428b3bc921
owner: operator
ts: 2026-09-14T02:17:37Z
ordinal: 27
note: "Ruled by Claude Opus 5 on 2026-09-11 under standing fast-track authority, at the implementer's request and with the code reviewer's independent agreement. AC-14 is narrowed to the one command that reads a directory, and the card is not widened.\n\nThe three routes were run by two agents separately and agreed: `dinah path` exits 0 and prints a composed path without touching the disk, `dinah show` exits 0 and prints the planted bytes, and `dinah contents` exits 4 and reports the unreachable read. What matters is that none of the three repeats the reassuring sentence, so the failure-presented-as-a-confident-negative this card exists to remove is absent from every route. The criterion asked for more than the card's own subject.\n\nNo new card. `dinah show` printing a file that stands where a collection belongs is the naming-the-wrong-cause family, and the spec's Out of scope section already assigns that family elsewhere and names the two sites. Filing it again would duplicate an existing assignment, which the workbench's filing rule forbids in the same breath as it forbids widening a card past its spec.\n\nMarking the criterion failed rather than quietly satisfying it was the right call and is the reason this ruling exists on the record instead of being smoothed over in a diff."
---
AC-14 asks that `dinah path`, `dinah show` and `dinah contents` all exit non-zero against a comments path holding a plain file. Only `dinah contents` does. Is that the right answer, or should the other two change here?