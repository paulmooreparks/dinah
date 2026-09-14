---
kind: open_question
state: resolved
column: c9428b3bc921
owner: operator
ts: 2026-09-14T02:17:04Z
ordinal: 43
note: The subsection says the reviewer can re-run the duplicate-English-text grouping because "internal/msg's TestATranslationTracksItsEnglishSource builds the same roster of duplicate-text keys as a side effect". It does not. That test runs one subtest per language tag and walks Keys() checking each entry's source fingerprint; it never groups keys by English text and its subtest names are language tags. I landed the text verbatim because AC-11 and AC-13 both require verbatim and the reviewer verifies by literal comparison, and I am flagging it rather than silently editing governance text. The recommended correction is to drop the parenthetical and point the reviewer at the grouping script recorded in this card's spec, which is the only place that method exists. Until then a reviewer following the instruction will run a test that answers a different question.
---
AC-13's verbatim text, now live in Agent Code Review's column instructions, contains a claim about TestATranslationTracksItsEnglishSource that is not true. Rule on whether to correct it.