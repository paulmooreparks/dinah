---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:50Z
ordinal: 39
note: Taken per command rather than as a principle, and the reasons differ by command; the uniform answer is a conclusion rather than an assumption. Claim, Release, Unblock and Archive are refused for reasons belonging to the individual row. Block applies one reason and a card already blocked does not change what the rest need. Pull acts on a column at a time. Attach File puts one file on several owners. Delete Attachment deletes one file at a time. Move is the only one where stopping has an argument, since a refusal can belong to the destination and recur down the whole selection; it continues anyway because the extension does not model which refusals are destination-wide, and deciding that it knows would be predicting the verb's answer, which src/dragAndDrop.ts's header already rules out for the drag path on the same grounds. Nothing is undone because dinah has no transaction spanning two references.
---
Every bulk command continues past a refusal, the successes stand, and nothing is rolled back.