---
title: The English help no longer says edit, path and show accept a state, though they do
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
---
dinah-151 was titled "dinah help show names the state kind it already accepts". It added the word to three argument summaries, because the commands take a state and the help did not say so. dinah-192 later regenerated the English, German and Hindi catalogs from templates and silently reverted all three. The five untranslated placeholder catalogs were never regenerated, so they still carry the sentence dinah-151 wrote.

The result, verified against `origin/main` across all eight catalogs:

```
param.edit.card.summary   en  ...`workbench` or `.`, a card, or something below a card...
                          af  ...`workbench` or `.`, a state, a card, or something below a card...
param.path.card.summary   same divergence
param.show.card.summary   en  a card, or something below a card...; show does not take this workbench
                          af  a state, a card, or something below a card...; show does not take this workbench
```

`af`, `cs`, `es`, `fil` and `id` all carry the correct wording in all three entries. English and Hindi have lost it in all three. German is the odd one: it lost it for `edit` and `path` but still says "ein Zustand" for `show`, because dinah-211 restored the German by hand after dinah-192 wiped it and caught one of the three.

**So the English is the catalog that is wrong, and the untranslated copies are the ones telling the truth.** That is worth stating plainly because it inverts the working assumption on this board. English is authoritative in the sense that everything is translated from it and the operator can read it; it is not thereby correct. Here it is stale and five machine copies of an older English are accurate.

**The fix.** Restore the wording in `en.json` and `hi.json` for all three keys, and in `de.json` for `param.edit.card.summary` and `param.path.card.summary`. Do not touch the five skeletons, which are already right. Confirm the behaviour before restoring the words rather than trusting dinah-151's title: check that `edit`, `path` and `show` genuinely resolve a state today, because the reverted sentence has been wrong for months either way and the question is which direction to correct it in. If any of the three no longer accepts a state, the fix is the opposite one and the skeletons need correcting instead.

**Why it matters beyond three strings.** This is a fourth failure mode, distinct from the three dinah-249 documents. Not a key left in English, not an entry differing by case, not a translation falling behind English, but English itself falling behind the tool while translations of an earlier, more accurate English survive. A checker that treats English as the authority and compares translations against it passes this cleanly while reading a stale authority. dinah-252's semantic layer is the only proposed check that could catch it, and only if it compares strings against behaviour rather than against each other.

Found by the implementer of dinah-248 while surveying entries whose English enumerates a set, and independently by Agent Design Review on dinah-249, which found the same three entries from the opposite direction while testing that card's claim about the skeleton catalogs. Two agents arrived at it by different routes on the same afternoon, which is the only reason it surfaced at all.
