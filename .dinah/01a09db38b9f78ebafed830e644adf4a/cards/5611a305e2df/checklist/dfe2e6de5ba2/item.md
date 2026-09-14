---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:17:41Z
ordinal: 12
note: "Armed by changing the catalog key column.columns.work from Work to Working. Red: \"internal/guide/guides/principles.md:29: the table's header row is not the one `dinah columns` draws:\", then \"the guide draws:   Slug    Name    Kind    Cards  Work        Owner\" and \"the tool draws:    Slug    Name    Kind    Cards  Working     Owner\" on separate lines. The comparison is by field through linesAgree, so padding differences do not fire and a renamed or added column does. Catalog restored, green."
---
A guide table whose header the tool no longer draws is caught. Arming: change the `text` of the catalog key `column.columns.work` in `internal/msg/locales/en.json` from `Work` to `Working`, run `go test ./cmd/dinah -run TestEveryGuideBlockShowsWhatItDeclares`, and watch it fail naming `internal/guide/guides/principles.md`, line 29, the block's header row, and the tool's header row on separate lines. Restore the catalog entry and watch the run go green.