---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:18:42Z
ordinal: 5
note: "Verified 2026-09-12 on head 995636e: go test ./internal/bench -run TestResolveCardReadsTheRegistry passed within the 41-test -run match. The five arms the criterion names all held: a claimed number resolves in the half being read, an unclaimed number and a tombstone-only number refuse unknown-card, a number two lines claim refuses dinah.ambiguous-card naming both claimants, and a 12-hex reference resolves without the registry being consulted."
---
Resolution reads the registry and both unchanged cases hold. `go test ./internal/bench -run TestResolveCardReadsTheRegistry` passes: a number one line claims resolves to that card in the half being read, a number no line claims refuses `unknown-card`, a number claimed only by a tombstone refuses `unknown-card`, a number two lines claim refuses `dinah.ambiguous-card` naming both identifiers, and a 12-hex reference resolves without the registry being consulted.