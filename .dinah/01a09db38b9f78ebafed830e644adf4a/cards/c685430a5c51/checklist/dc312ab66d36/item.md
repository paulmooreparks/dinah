---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:07Z
ordinal: 15
note: "`checkEveryShapeSaysWhatToDoNext` in internal/profile/guards_test.go:3140 fails a shape whose last NextStep member carries a When, an Unless or a WhenCommand, on the ground that a conditional last member leaves a reader whose values match no branch with no next step at all. The parent's next-item and next-attachment are both conditional, so a third member was required whatever the workstream did, and the workstream is the reader who renders it today. Its text names the four kinds that do keep attachments, so it is useful advice rather than a placeholder."
---
A third, unconditional next step is minted beside the parent's two, because the shape guard requires the alternation's last member to carry no condition.