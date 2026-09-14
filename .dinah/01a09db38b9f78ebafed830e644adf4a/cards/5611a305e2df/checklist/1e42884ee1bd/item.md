---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:17:40Z
ordinal: 2
note: Re-armed independently by Test on 56db064. Deleting the ledger line `docs/quick-start.md:561 figure=Five noun=commands counts=the verbs that change where a card stands derives=contractVerbs` and running `go test ./cmd/dinah -run TestEveryProseFigureIsDeclared` fails with "docs/quick-start.md:561 says Five commands and testdata\prose-figures.txt carries no entry for it; write one saying what the figure counts and what holds it", naming the document, line 561, the figure Five, the noun commands and the ledger file as the criterion requires. Entry restored and the five prose checks green.
---
An undeclared prose figure is caught. Arming: delete the ledger entry for `docs/quick-start.md:561`, run `go test ./cmd/dinah -run TestEveryProseFigureIsDeclared`, and watch it fail naming the document, line 561, the figure `Five`, the noun `commands`, and `cmd/dinah/testdata/prose-figures.txt` as the file to write the entry in. Restore the entry and watch the run go green.