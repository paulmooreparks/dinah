---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:39Z
ordinal: 10
note: The spec takes the shape of the guide's `dinah columns` listing from a `dinah status` block in the quick start, on the ground that both heads render through `renderColumns` (cmd/dinah/render.go:256, called from commands.go:550 and render.go:204). That is a claim about behaviour, so this criterion asks the tool instead of asking the paragraph. It also covers the Work values, which AC-6's heading-row comparison does not reach.
---
In a scratch directory under C:\dinah-scratch with `DINAH_HOME` pointed inside it, `dinah init` followed by `dinah columns` prints a heading row reading `Slug  Name  Kind  Cards  Work  Owner` and a `Work` cell reading `none taken` on the intake and done columns and `taken` on the work column, and the corrected block at `internal/guide/guides/principles.md:30-35` carries that heading row and those three values.