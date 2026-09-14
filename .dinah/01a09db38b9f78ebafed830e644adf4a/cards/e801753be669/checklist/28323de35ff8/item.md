---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:32Z
ordinal: 14
note: Verified by internal/verb/link_test.go:TestNoVerbRefusesOnAccountOfALink, which claims the card as bob, fails loudly if the holder is not bob, then has alka both link and unlink on it and asserts the holder is still bob afterwards. Neither verb references the claim system anywhere in internal/verb/link.go.
---
Both verbs succeed on a card claimed (held) by a different actor, exactly as `comment` does; neither checks or touches the claim/lease system.