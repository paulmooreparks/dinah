---
kind: decision
state: resolved
ts: 2026-09-14T02:18:04Z
ordinal: 21
note: cmd/dinah/testdata/guide-blocks.txt declares every fenced block of every guide by guide and by the line its opening marker stands on, and a block nobody declares fails. references.md has no entry there today because it carries no fenced block, so keeping every block indented leaves a line-number-keyed ledger untouched by a rewrite that moves every line in the file.
---
The guide keeps its four-space indented blocks and gains no fenced block.