---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:30Z
ordinal: 3
note: "Re-verified independently on Test cycle 2026-08-27, on the re-merged tree at a50b68c. `go test ./cmd/dinah/ -run TestTheCompactListingCarriesEveryFieldTheCanonicalListingCarries -v -count=1` passes all five listing subtests, env cleared. Armed by swapping card.Severity and card.Priority in compact.go's field-order table: two subtests (ls, ls doing) reddened, printing \"priority\":\"major\" on the compact side where canonical carried \"severity\":\"major\" (and vice versa), naming the transposition exactly. Reverted, confirmed green. Also manually ran the tool end to end: filed a card with a title containing a pipe, a backslash and an embedded newline, claimed/blocked/unblocked a second card, and confirmed --format compact ls / --json ls agree on every field including the escaped title and the ready/active/blocked condition value."
---
For a fixture bench carrying at least: one card whose title contains a literal pipe, a literal backslash, and an embedded newline; one card belonging to two workstreams; one card with severity and priority set; one state holding zero ready cards, --format compact ls and --format json ls decode (via the test-only compact decoder) to identical CardView field values per card, in identical order, for both the whole-bench listing and a single-state listing.