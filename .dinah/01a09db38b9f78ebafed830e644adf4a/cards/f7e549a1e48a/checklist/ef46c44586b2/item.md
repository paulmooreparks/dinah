---
kind: acceptance_criterion
state: pending
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:20Z
ordinal: 3
---
`crypto/rand.Read` is documented never to return an error on any platform Dinah ships for, and its own documentation states that the one platform where the underlying call can fail (a pre-3.17 Linux kernel, before /dev/urandom is seeded) crashes the process rather than returning control to the caller. There is therefore no reachable path where a claim survives a failed mint to refuse on it, and no test can construct one. This criterion is downgraded to a code-reading check: the diff must not silently discard the error Read's signature returns and proceed with a blank or zero discriminator standing in for a real one. Where the chosen combination is lettered option C (Change 1 alone, no conflict-refusal built on the discriminator yet), even this reading-only check is deferred to the card that builds Change 3, which the spec should say plainly rather than leaving silently unverified.