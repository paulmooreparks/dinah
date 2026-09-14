---
kind: decision
state: resolved
column: 4b38abe7ebd5
ts: 2026-09-14T02:17:53Z
ordinal: 38
note: "No, and the two questions are held apart in the code as well as in this note. A segment is how a reference spells the collection it narrows; a kind is what the format stores in an item's frontmatter, what `dinah file` takes as an argument, and what the JSON payload prints. The rename is addressing, so the tokens are untouched: `dinah file fx-1 open_question ...` is unchanged, and the payload still carries all three tokens. checklistSegments carries the Kind and the Word as separate fields with a paragraph on the declaration saying a segment is not a kind, and AC-16 pins the boundary by planting the confusion itself, filling ItemView.Kind from WordForItemKind and watching the payload assertion redden."
---
Does the rename reach the kind tokens `open_question`, `acceptance_criterion` and `decision`?