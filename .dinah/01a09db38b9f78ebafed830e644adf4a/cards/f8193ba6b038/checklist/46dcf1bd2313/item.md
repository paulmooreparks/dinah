---
kind: open_question
state: resolved
column: aa6cd1c6ae5f
owner: operator
ts: 2026-09-14T02:16:32Z
ordinal: 23
note: "Recommendation: versioned rather than frozen. The canonical JSON form earned its freeze by being the one thing every conformance suite and every consumer keys on; the compact form is new, unproven at scale, and this spec's own token-saving numbers (AC-9) are estimates pending a real tokenizer run. Freezing it now risks freezing a mistake. The version record (D-5 above) already carries the mechanism for either answer: a frozen posture just means the board commits to never incrementing past 1, and a versioned posture means it can. Recorded here rather than decided, because a stability promise to whatever reads this format is exactly the kind of commitment this spec's own governing instructions call durable and awkward to reverse."
---
Is the compact format a frozen contract for the life of the profile, the way the canonical JSON form is, or a versioned surface that may change behind its own fmt|compact|N marker as the board learns more about what a driver loop actually needs?