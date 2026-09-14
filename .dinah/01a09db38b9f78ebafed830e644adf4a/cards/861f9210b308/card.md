---
title: A count in a sentence needs a plural form, and no catalogue entry has ever had one
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
A message now reads "contains 1 entities". Fixing it properly means the first plural-aware entry this repository's message catalogues have ever carried, in all eight languages, which is why it is a card rather than a wording nit. It was found by code review on dinah-455 and left there deliberately, with the implementer arguing it is real work and the reviewer agreeing.

Two things make this bigger than the sentence that exposed it.

A sibling string one line above has the same defect and stays wrong if only the reported one is fixed. So the card is the mechanism, not the instance, and whoever takes it should sweep every reader-facing string that interpolates a count rather than fixing the two that are known.

Plural rules are not the same in the eight languages Dinah speaks. English and German need two forms. Hindi needs two but draws the boundary differently. Several of the five skeleton languages have rules that differ again, and a skeleton entry today is byte-identical English, which cannot express a form English does not have. So this reaches the translation staleness contract and the guards that enforce it: what a skeleton means for a plural entry has to be settled before any text is written.

What the card should settle, none of it the operator's to rule on:

Whether the catalogue gains a plural mechanism or whether the strings are rewritten to avoid counting inside a sentence. The second is cheaper and is a real option: "contains 1 entities" also disappears if the sentence names the count in a form that does not inflect. Weigh it honestly rather than reaching for the general mechanism because it is more interesting, and remember DIRT is a debt test rather than a mandate to build the larger thing.

If a mechanism lands, what a skeleton language carries, and what the existing parity and fingerprint guards do with an entry whose shape differs per language. Those guards currently compare keys and source fingerprints across catalogues, and a plural entry is the first thing that would not be one string per key.

Which strings are in scope, established by sweeping the catalogues rather than by listing the ones already reported.

Filed from dinah-455, which is where the sentence appeared. That card ships the sentence as it stands rather than growing to cover this.
