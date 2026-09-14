---
title: German text agrees with a value it cannot see, beyond the class one card fixed
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
`reshape.carry` in the German catalogue reads `{id} schickt seine Karten an {destination}`. The possessive picks a gender, and what it points at is a column reference, which has none: the placeholder holds an untranslated token, so no German word can legitimately agree with it.

This is the same defect dinah-460 wrote a rule against, one step wider than the class that card bounded. That card enumerated the seven catalogue entries interpolating a kind, fixed the two German ones that were wrong, and wrote the rule into all eight catalogues. This entry interpolates something else, so it sits outside those seven and that branch does not touch it. Found by code review on dinah-460, which read the class in German rather than counting it, and named this as wanting its own card.

Why it is worth one rather than a quiet edit. The rule dinah-460 wrote is stated for a class it bounded, and the defect is plainly wider than that class, so the question is what the rule actually is and where it applies. Answering that is the card; changing one sentence is not.

What the card should settle, none of it the operator's to rule on:

The real boundary. Every placeholder in every catalogue either holds text in the reader's language or holds a token that stays as it was written, and only the first can be agreed with. Enumerate the second kind off the source rather than by reading translations, then check every catalogue entry that interpolates one. Do not start from German; the defect is about what a placeholder holds, and the language it shows up in first is an accident.

Whether a guard is possible, and be honest if it is not. dinah-460 deliberately shipped no committed guard over German, because the same pattern across the whole catalogue fires five times and is wrong every time, all five being the German preposition "an", in a language nobody on this board can adjudicate a red for. That ruling was made twice and endorsed by review both times, so a guard here needs to beat it rather than ignore it. A guard that fires falsely in a language its readers cannot judge is worse than no guard.

What the five skeleton languages mean for this. They carry byte-identical English, so they cannot have the defect today and will acquire it the moment anybody translates them. Say whether the rule reaches them and how a future translator learns it.

Related: dinah-460 bounded and fixed the narrow class and wrote the rule; the article-agreement class is already in the convention counterexamples corpus, promoted after two catches, and this is a third of a related shape.
