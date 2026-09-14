---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:21Z
ordinal: 23
note: It reads "or a comment such as wb-1/comments/1", implying `attachments` takes only a comment below a card. At 808d105 `attachments wb-1/attachments/1` and `attachments wb-1/checklist/1` both exit 0, so it takes anything below a card. Recorded separately from the enumeration strip so a reviewer does not read it as one.
---
`param.attachments.ref.summary` loses a narrowing that was itself false, which is a correction rather than a strip.