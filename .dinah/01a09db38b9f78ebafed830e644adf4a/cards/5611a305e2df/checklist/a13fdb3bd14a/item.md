---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:17:40Z
ordinal: 9
note: Armed with the exact paragraph the criterion names, appended to internal/guide/guides/getting-started.md. Both checks stayed green, so a figure standing beside workbenches, cards, or column raises nothing. Paragraph removed. The guard sees twenty-three of the corpus's 385 figures by design, and this is the criterion that shows the other 362 do not cry wolf.
---
The prose guard does not cry wolf. Arming: append to `internal/guide/guides/getting-started.md` a paragraph reading "Open two workbenches at once and Dinah keeps three cards apart, since one column holds four of them and the second holds none.", run `go test ./cmd/dinah -run 'TestEveryProseFigureIsDeclared|TestNoProseFigureEntryIsStale'`, and watch it stay green. Remove the paragraph. The verdict is that a figure standing next to an unregistered noun raises nothing.