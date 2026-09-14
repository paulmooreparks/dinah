---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:18:43Z
ordinal: 9
note: "Verified 2026-09-12 on head 995636e: go test ./cmd/dinah -run TestRenumberRepairsTheLaterClaimant passed in 0.44s. Over the registry carrying two lines claiming one number, dinah check --renumber --yes left the earlier line untouched, rewrote the later line in place with the next number above the high-water mark, left every other line at the same index, wrote a renumbered event on the renumbered card's journal, reported check.card-number-renumbered naming that card, and a following dinah check reported no duplicate."
---
The repair renumbers the later claimant and leaves line order alone. `go test ./cmd/dinah -run TestRenumberRepairsTheLaterClaimant` passes: over a registry carrying two lines claiming one number, `dinah check --renumber --yes` leaves the earlier line untouched, rewrites the later line in place with the next number above the high-water mark, leaves every other line at the same index in the file, writes a `renumbered` event on the renumbered card's journal, reports `check.card-number-renumbered` naming that card, and a following `dinah check` reports no duplicate.