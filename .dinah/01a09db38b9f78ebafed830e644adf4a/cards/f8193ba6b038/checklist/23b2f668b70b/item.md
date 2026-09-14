---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:30Z
ordinal: 4
note: "Re-verified independently on Test cycle 2026-08-27, on the re-merged tree at a50b68c. `go test ./cmd/dinah/ -run TestTheCompactOffersCarryEveryFieldTheCanonicalOffersCarry -v -count=1` passes, env cleared. Manually ran `--json next` and `--format compact next` against a three-column scratch workbench (a column with a ready card, a column with a ready card not taken by pull, and a column offering nothing): the compact off/card records decode to the same taken_by_pull / no_taker / card-presence values as the canonical JSON, in the same column order."
---
For a fixture bench carrying at least one state with a ready card to offer, one state with nothing ready, and one state with AwaitingOutside set, --format compact next and --format json next decode to identical Offer field values per state, in identical order, including the presence or absence of a card on each offer.