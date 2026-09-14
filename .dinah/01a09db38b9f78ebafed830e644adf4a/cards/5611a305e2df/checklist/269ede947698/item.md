---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:17:40Z
ordinal: 10
note: Re-checked independently by Test on 56db064. The three guide-block checks are green, and cmd/dinah/testdata/guide-blocks.txt carries 21 entries at lines 30-50 breaking down as 13 shows=json, 5 shows=commands, 1 shows=tree, 1 shows=shell and 1 shows=table, matching the criterion exactly. A bare `grep -c shows=` reports 22 because line 10 of the header comment explains the key; that is the grep being loose, not a 22nd entry.
---
The guide block ledger and its three checks are green on an unmodified tree, and the ledger declares all 21 fenced blocks the eight embedded guides carry. Verdict: `go test ./cmd/dinah -run 'TestEveryGuideBlockIsDeclared|TestNoGuideBlockEntryIsStale|TestEveryGuideBlockShowsWhatItDeclares'` exits 0, and `cmd/dinah/testdata/guide-blocks.txt` names thirteen `shows=json` blocks, five `shows=commands` blocks, one `shows=tree` block, one `shows=shell` block, and one `shows=table` block.