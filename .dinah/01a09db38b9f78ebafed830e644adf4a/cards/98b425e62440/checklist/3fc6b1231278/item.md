---
kind: open_question
state: resolved
column: c9428b3bc921
owner: holder
ts: 2026-09-14T02:16:36Z
ordinal: 18
note: Resolved by splitting parseArgs's inner loop into a shared walkFlags function that both parseArgs and scanLangFlag call, fed the same valuedFlags/markerFlags. Neither caller pattern-matches the literal token "--lang" against raw argv any more; walkFlags is the one place that decides whether a word is a flag's own value, so a --lang sitting in a preceding valued flag's value slot (dinah move card1 --state --lang de) is consumed by that flag's occurrence inside walkFlags itself and is never visited as name=="lang" by scanLangFlag's own callback. See the card spec, "Reading argv once" and "Why the two callers now agree everywhere, not just on the cases already tested".
---
scanLangFlag matches the literal token `--lang` (or `--lang=X`) name-blind, so it misfires when that token actually sits in the value slot of a preceding valued domain flag (e.g. `dinah move card1 --state --lang de` sets state's value to the literal string "--lang" in the real parse, leaving no --lang flag given at all, yet scanLangFlag reads it as --lang=de). Re-spec scanLangFlag to walk argv using the same known/valued-flag tracking parseArgs uses, so it skips a value slot instead of pattern-matching the bare word.