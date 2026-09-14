---
kind: decision
state: resolved
ts: 2026-09-14T02:17:19Z
ordinal: 7
note: "Confirmed by reading dinah-209 and internal/verb/mutate.go's canRoute/canLand, neither of which reads anything but Position today. Stronger than first stated: Position is not merely undisturbed by lanes, it is architecturally a single running index. bench.go's opener reads one sequence key (`ids := fm.Seq(vocab.SequenceKey)`, bench.go:1543) and assigns each column's Position as `len(b.Columns)` while walking it in order (bench.go:1559, `readColumnIn(root, vocab, id, len(b.Columns))`). A workbench anchor has exactly one such sequence today, so the on-disk format cannot declare a second route to be indexed against, not merely that no workbench happens to use one. This card's read of \"columns skipped\" as a Position difference holds for every workbench the current format can express."
---
This card does not depend on dinah-209 (lanes) landing first. With one route per workbench, Position alone gives a total order, and "columns skipped" is destination.Position - departure.Position - 1 on a forward move, no lane concept required.