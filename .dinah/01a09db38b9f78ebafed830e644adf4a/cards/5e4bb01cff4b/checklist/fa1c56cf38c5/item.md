---
kind: decision
state: resolved
ts: 2026-09-14T02:17:02Z
ordinal: 25
note: "Every entry in en.json whose context carries dinah-258's alternate phrase \"the same in every language\" (refusal.dinah.unknown-field.ordered, token.dinah-editor, token.visual) was checked by hand against all four seeded glossary triggers (state, the root, owner, level); none of the three texts contains any of them, so no entry fails today because of the gap. dinah-258 is a narrow, already-scoped fix to an existing test sitting unclaimed in Intake; this card does not depend on it landing first, since the shared phrase behaves correctly for every entry that exists now. Left as a note for whoever implements dinah-258: if that fix turns the phrase-selector into a shared structured signal rather than a second literal string, this glossary guard's exclusion should read the same signal so the catalog keeps one definition of \"declared untranslatable\" instead of two that can drift apart."
---
The glossary guard's "never translated" context exclusion shares dinah-258's incomplete-selector risk, but that risk is live-checked as zero today, and the two cards stay separate rather than being merged.