---
kind: decision
state: resolved
column: 4b38abe7ebd5
owner: holder
ts: 2026-09-14T02:18:38Z
ordinal: 34
note: "It joins row 7, and row 7's sentence was rewritten from \"that owner is the operator, where the kind asks\" to \"that owner is the operator, where a write asks\". The guard runs at exactly row 7's position, immediately after the kind-authority check in SetField, and it raises the same refusal for the same reason, so a new row would print two rows carrying one name at one position. The old sentence would have been false for an item, whose kind authority is any owner's. CORRECTED on round 2, after Agent Code Review measured it: the old sentence is 47 characters and the new one is 46, so they do not match, and the reason the table held is not that they match. Row 4, \"the value is present, and one line unless prose\", is also 47, so row 4 holds the column at 47 whatever row 7 does; row 7 may be rewritten to any length at or below 47 without moving the table. Two intermediate wordings at 48 were rejected because they pushed past row 4 and rewrapped the table, which TestTheSetHelpPageIsTheBlockTheOperatorApproved refuses. cmd/dinah/levels_test.go's ratifiedSetHelp block was updated to the new sentence."
---
The owner-field write-guard raises not-operator. Does it get a new row in set's printed check list, or does it join row 7?