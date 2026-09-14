---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:30Z
ordinal: 5
note: Re-verified independently on Test cycle 2026-08-27, on the re-merged tree at a50b68c. `go test ./cmd/dinah/ -run 'TestEveryAcceptedActDecodesTheSameUnderBothMachineForms|TestEveryRefusedActDecodesTheSameUnderBothMachineForms|TestTheCompactFormReachesBothResponseCallSites' -v -count=1` all pass, env cleared. Manually ran claim/block/unblock against a scratch card and confirmed compact and json responses agree field for field, including the Instructions/LegalMoves/Holder/ClaimSince fields on the OK claim outcome.
---
For claim, move, release, block and unblock, each exercised once to an OK outcome carrying Instructions and at least one LegalMove, and once to a refused outcome carrying Context, --format compact and --format json decode to identical Response field values, including Card, Instructions, LegalMoves, Context, Affordances, Message and MessageValues where each is present.