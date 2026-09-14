---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:40Z
ordinal: 7
note: "Whole-suite gate passed: go test ./... -count=1 on the merged tree at df6f0f0 (merge with origin/main at 933f0d5 reported Already up to date, so the tested tree is exactly the branch head) exited 0 with all fourteen test-bearing packages ok and no FAIL. A focused go test -v -run re-run printed --- PASS for all thirteen new tests with no --- SKIP, including the guard's six family subtests A through F. The five guards this criterion names are green inside the sweep: internal/contract covers Introduced and Shapes, internal/msg covers both keys in all eight catalogs, and cmd/dinah's address_form_arms_test roster-checks resolveCardIn Returns 9 and ResolveLinkTarget Returns 7 with Accepting 2 and 3 on this branch."
---
`go test ./...` passes on the branch merged with current trunk, with the whole suite run rather than the touched packages. That run covers the five guards this change moves: contract.Introduced carries dinah.ambiguous-card, contract.Shapes declares it, all eight catalogs carry both new keys with Present equal to Total, internal/addressform declares Returns 9 for resolveCardIn, and internal/addressform declares Returns 7 for ResolveLinkTarget with Accepting unchanged at 3.