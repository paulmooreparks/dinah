---
kind: decision
state: resolved
ts: 2026-09-14T02:17:20Z
ordinal: 9
note: "Re-checked against dinah-387's actual spec text (not the description). dinah-387 never proposes touching canLand (mutate.go:359-426). Its described fix space is claimableState (mutate.go:230-238, called only from the claim verb via canClaim, not from canLand/move/pull) and claim() itself (mutate.go:243-253); its option 3 changes claimableState's own comparison. dinah-387's spec misfiles these as checks.go:230-237/234-236, but they are in mutate.go at those line numbers; checks.go is an unrelated file (a refusal-key lookup table). Conclusion is unchanged on the corrected facts: dinah-387's real fix space (claimableState/claim, ~218-253) still does not overlap dinah-376's (canRoute 318-335, legalMoves 500-527, the moved event's field list), and canLand's own held-check at 363-364 is untouched by either card. No overlap either way, but the note's claim about which function is dinah-387's territory was wrong and is corrected here."
---
dinah-376 and dinah-387 do not overlap in fix space even though both touch move/journal code. dinah-387's territory is the held/claim rows in canLand and Card.Holder; this card's is canRoute, the loop in legalMoves, and the moved event's field list.