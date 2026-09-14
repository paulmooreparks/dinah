---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:39Z
ordinal: 8
note: The exemption fixture is the remaining line-number-keyed fixture on this workbench. The spec's edits are line-count neutral at and above line 1367, so this should pass untouched; the criterion exists because nothing announces a drift and an entry that lands on a neighbouring fence can still pass the suite.
---
Every line number named in cmd/dinah/testdata/quickstart-exempt.txt still opens a fence in docs/quick-start.md, checked by comparing the entry numbers against `grep -n '^```' docs/quick-start.md`, and `go test ./cmd/dinah/ -run TestEveryExemptBlockDeclaresTheCatalogEntriesItQuotes` passes.