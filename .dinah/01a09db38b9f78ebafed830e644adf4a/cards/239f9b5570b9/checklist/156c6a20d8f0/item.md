---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:52Z
ordinal: 25
note: "The two card blocks are being brought into line with the checklist block, which dinah-456 section 4.3 names as the model they follow, so they take that block's key segment and its English word rather than a third spelling. The workstreams listing is a standalone listing rather than a block of `dinah show`, and the other standalone listing carrying an address, the containment tree, uses column.tree.reference reading \"Reference\"; the listing joins that one. Both fingerprints are certain rather than guessed: msg.Fingerprint(\"Ref\") is 9ff1e019feac1ee2 and msg.Fingerprint(\"Reference\") is fe88d270aa3e71da, computed with FNV-1a 64 and matching the source values column.checklist.ref and column.tree.reference already store in de and hi. This item carries the design argument for the key names and nothing else. The per-key translation record the staleness contract requires is D-16, D-17 and D-18, one item per key, which is the ordinary branch of that document; round 1 recorded all three in this item, which is the collective branch the document reserves for a diff filling hundreds of skeleton entries."
---
The two new heading keys are column.comments.ref and column.attachments.ref reading "Ref", and the workstreams listing takes column.workstreams.reference reading "Reference".