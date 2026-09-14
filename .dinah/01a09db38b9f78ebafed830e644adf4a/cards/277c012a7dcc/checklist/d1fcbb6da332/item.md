---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:19Z
ordinal: 4
note: "Verified. The same test renders each page at COLUMNS=40 and COLUMNS=200 and requires the two folded clauses to be equal; the assertion is the t.Errorf reading \"draws the kinds clause as %q at COLUMNS=40 and %q at COLUMNS=200\". Plant recorded honestly, because the spec's predicted plant needed correcting: a plant that dropped the clause whenever it exceeded the width dropped it at BOTH widths and therefore reddened the earlier \"carries no row opening\" arm rather than this one, which proves nothing about the two-width comparison. The plant that exercises this criterion makes the renderer differ BETWEEN widths, drawing the full clause when it fits and a one-kind clause when it does not. That reddened naming the command and both widths, e.g. \"`dinah help archive` under af draws the kinds clause as \\\"a card\\\" at COLUMNS=40 and \\\"a workstream, written as `workstream/<slug>`; a column; a card; something below a card\\\" at COLUMNS=200\". Restored byte-identically and green."
---
The same test renders every page twice, at `COLUMNS=40` and at `COLUMNS=200`, and requires the two folded clauses to be equal; a plant that makes the renderer emit the clause only when it fits one line reddens naming the command and both widths.