---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:10Z
ordinal: 18
note: "`dinah workstream get <ref>` with no field prints a field table rather than a value, so retiring the spelling retires a read as well. Probed at 9260a2a, the bare `dinah workstream` listing carries every column that table carries, identifier included in its JSON, so the retirement costs the narrowing rather than the information: the notes are `dinah get workstream/<ref> notes`, the member cards are `dinah query workstream:<ref>`, and the directory is `dinah path workstream/<ref>`. A bare `dinah workstream <ref>` form was considered and rejected because it would make `dinah workstream get autumn status` resolve as a workstream named `get` and refuse `dinah.unknown-workstream`, which is a worse answer for a reader typing the retired spelling than the `dinah.usage` refusal dinah-456 section 5.2 asks for. Widening `dinah show` is the honest way back and dinah-456 D-25 already puts it outside this contract. Spec section 7.4 carries both transcripts."
---
The workstream detail read goes with `workstream get`, and no bare `dinah workstream <ref>` form is invented to replace it.