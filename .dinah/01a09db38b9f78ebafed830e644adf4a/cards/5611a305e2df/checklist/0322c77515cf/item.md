---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:17:40Z
ordinal: 7
note: "Armed. Red: \"derivationsByName registers configKeys and no entry in testdata\\prose-figures.txt names it, so a derivation stands in this file that nothing reads\". Restored, green."
---
A derivation no entry reads is caught. Arming: delete the ledger entry for `docs/quick-start.md:222`, which is the only entry naming `configKeys`, run `go test ./cmd/dinah -run TestEveryLedgerReferenceIsLive`, and watch it fail saying that `derivationsByName` registers `configKeys` and no entry names it. Restore the entry and watch the run go green.