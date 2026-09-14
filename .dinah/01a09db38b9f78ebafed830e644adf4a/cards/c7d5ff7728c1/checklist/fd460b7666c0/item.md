---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:01Z
ordinal: 34
note: "The two are different kinds of text and the rule splits between them. Sections 3.1, 3.2 and 4.3 are the contract, which is what the tool will do, so they carry `questions`, `criteria` and `decisions` with the short forms accepted on input. Sections 2.3 to 2.6 are probe transcripts of a binary built at 22a35fc, which is what the tool did, and AC-5 checks them by re-running against that commit, so changing a character in them would make a true record fail its own check. Section 2.1 is where the two meet and it says both. Verified against the trunk this round rather than assumed: `checklistKinds` in internal/bench/resolve.go still reads oq / ac / d at 813e0bb, and the rename has not yet landed on dinah-454's branch either, so the contract is ahead of the code by design and says so. Also verified that `itemKindAlias` derives the composing direction from that same map, which is why the rename is one edit rather than two and why the two directions cannot drift."
---
The checklist alias rename ruled on dinah-454 is carried into this contract's grammar and its quoted addresses, and the probe transcripts keep the spellings the binary actually printed.