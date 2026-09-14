---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:32Z
ordinal: 9
note: "Verified by internal/verb/link_test.go:TestLinkWritesNothingToTheCardItNames, which compares the named card's whole anchor byte for byte before and after, compares its journal length, and asserts its Links stays empty. Armed by making Link also write a reverse entry on the target: red naming the mirrored block and 'the named card carries [{Kind:blocks To:720e...}]'. Restored, green."
---
`link` never writes to the target card's own file: a link recorded on card A naming card B leaves card B's `links:` block, or its absence, exactly as it was before the call.