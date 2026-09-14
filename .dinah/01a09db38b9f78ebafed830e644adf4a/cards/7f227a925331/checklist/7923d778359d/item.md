---
kind: decision
state: resolved
column: 4fda9c9ca779
owner: holder
ts: 2026-09-14T02:18:43Z
ordinal: 15
note: "Resolved 2026-09-12: dinah-487 landed on trunk. origin/main's tip is 6fbb3fede62ab817b5cdba4053f9d6cbd1222a17, subject \"dinah-487: a card number that names two cards now refuses with dinah.ambiguous-card\", verified by git ls-remote and by a fetch of origin before this note was written. The hold this decision placed is discharged and this card's build may proceed against a tree carrying the ambiguity refusal."
---
Hold this card at Implement until dinah-487 merges to trunk. Both cards change resolution; dinah-487's refusal-on-ambiguity must land first so this card does not build into a conflict. The design stages (Spec, Agent Design Review, Operator Design Review) touch no code and may run in parallel with dinah-487.