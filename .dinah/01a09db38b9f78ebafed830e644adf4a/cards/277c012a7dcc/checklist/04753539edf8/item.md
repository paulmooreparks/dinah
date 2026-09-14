---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:21Z
ordinal: 19
note: "Substring matching would have to decide whether \"a card\" inside \"something below a card\" is a hit, and the answer is language-specific, which is the dinah-460 wall again. Splitting on `verb.ReferenceKindSeparator` (\"; \") is exact and needs no judgement, and `TestNoReferenceKindLabelCarriesTheClauseSeparator` refuses a catalogue that would break the split, in whichever language introduced it. Round 2 corrects what is split: the rendered row cannot be split whole, because two of the ten new summaries carry the separator themselves, so the check first renders that catalogue's `help.reference-kinds` entry with the known summary and a sentinel for the kinds, takes the literal prefix and suffix around the sentinel, and splits only the region between them. The label ban therefore stays on the `reference.kind.*` labels alone, which round 3's sixth kind takes from five to six, and the remedy for a translator who breaks it is one punctuation change rather than an exemption entry."
---
The kinds are split out of the rendered row by reversing the template that wraps them and then splitting that region on a minted separator, rather than by matching labels as substrings, and no kind label in any language may carry that separator.