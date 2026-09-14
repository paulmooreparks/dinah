---
title: Nothing in the review loop compares two acceptance criteria against each other, so a contradiction between them ships
column: 5ea2db0272fc
state: ready
severity: major
priority: next
---
Every check this board's review columns run is a criterion against the code. Nothing compares two criteria against each other. So two criteria describing the same scenario can require opposite outcomes and pass every review, because each one is individually consistent with what the code does or should do, and no reader is asked to hold them side by side.

That is not hypothetical. dinah-409 carried two criteria for one log line, one requiring it to print the raw column identifier and saying explicitly it must not be a resolved title, the other requiring the resolved title and saying explicitly it must never be the raw identifier. Both gated Merge. No implementation could have satisfied both, so the card could never have left that column whatever anybody built. It survived a full round of an otherwise thorough review and was caught only on the round after.

Two things made it worse and both are worth carrying into the fix.

The contradiction was in plain English, not in a subtle interaction. Anyone reading the two items consecutively would have seen it. Nobody read them consecutively, because nothing in the loop asks for that.

And it survived partly because a handoff claimed the older criterion had been rewritten when the checklist item had never changed. A claim of having changed something was accepted as evidence that it had been changed. That is the same defect this board has spent weeks catching in other forms, arriving in the review process itself rather than in a card's content.

The spec author and the reviewer named this independently, on the same card, without conferring. The reviewer recommended a standing fix to the review column's own instructions rather than a note on the card.

What the fix has to decide. Whether this belongs in a column's instructions, where it binds every review that column runs, or in something a review can actually execute, since an instruction to compare criteria against each other is exactly the kind of instruction that gets read and not performed. Whether the comparison is worth running over every pair on a card or only over pairs naming the same artifact, since a card with nineteen criteria has a great many pairs and most of them are unrelated. And whether anything mechanical can help, given that two criteria contradicting each other is a question about meaning rather than about text.

Worth reading dinah-409's third and fourth review comments before specifying, since they carry the incident and both independent statements of the gap.
