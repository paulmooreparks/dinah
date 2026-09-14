---
title: The catalog guard only recognises one of the two ways an entry declares a name untranslatable
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
dinah-248 landed a guard holding every catalog to the machine identifiers its English source names. It selects which entries to check by reading each entry's translator note and matching the literal phrase "never translated", combined with the entry's text actually spelling an ALL-CAPS identifier.

Two entries in the same catalog declare the identical property in different words, "the same in every language". The guard skips them and does not know it skipped anything, which is the failure mode the guard itself exists to prevent, reproduced one level up.

Agent Code Review found this on dinah-248's second lap and judged, correctly, that it was not worth a third lap on that card: nothing ships wrong, all four exposed entries are correct in all eight languages today, so this is a limit on what the guard will catch later rather than a green test sitting on a live defect.

**The fix is one line of code and one sentence of prose.** Teach the selector the second wording. That enrols two entries immediately, both already green, so the change is free to make and provable by making it. Then add a declaration to the two entries that genuinely carry none.

**The correction that matters, because dinah-248's own handoff got it wrong.** That handoff says four entries name a real identifier without declaring it untranslatable, and proposes adding a note to all four. Two of them do declare it, in the other wording. A card written from that sentence would add redundant text to two entries and never touch the selector, which is the thing that is actually short.

**Which two are genuinely exposed, from the review's own reading.** The refusal that tells the reader to set the editor variable puts that name in the middle of a German imperative sentence, which is exactly where a translator would germanise it. And the MCP command summary is the string dinah-248 rewrote by hand; it is correct because the implementer was careful, not because anything checked it. The other two entries consist of nothing but the variable name, identical in all eight languages, where a translation would be obviously absurd rather than quietly wrong.

**Worth settling while in there.** The selector currently joins two filters that measure different things: one asks whether the entry declares something untranslatable, the other whether it contains a capitalised name. Neither asks the question the guard is about. It selects exactly one entry today because of what the catalog happens to look like, not because the rule derives it. Matching a second phrase is a patch on that, not a repair of it. Decide whether the declaration should become a structured field rather than a phrase in prose, and if that is out of scope here, say so and leave the reasoning for whoever takes it up.
