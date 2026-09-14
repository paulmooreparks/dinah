---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:33Z
ordinal: 20
note: guideProse keeps backticks and TestNoSentenceStandsInTwoGuides compares what it returns. Stripping inside guideProse was measured over the eight guides and introduces no new sentence collision, so it would be safe, but it changes another check's subject for no gain to this one. The strip raises this check's firing count from 12 to 14, so it is load-bearing where it sits.
---
The backtick strip is applied at the denial check's call site rather than inside guideProse.