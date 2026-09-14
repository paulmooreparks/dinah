---
kind: decision
state: resolved
column: c9428b3bc921
owner: holder
ts: 2026-09-14T02:17:02Z
ordinal: 28
note: "\"die genannte Werkbank liegt unter --root\" became \"die genannte Werkbank liegt im Wurzelverzeichnis\". The trunk moved under this card (dinah-282) and added a 52-display-column budget to the entry's own context, so the merge resolution keeps the trunk's context and uses a 48-column German rather than the 55-column \"unter dem Wurzelverzeichnis\" first written. English unchanged, so the existing source fingerprint stays correct and TestATranslationTracksItsEnglishSource passes."
---
de/check.mcp.2: read against the current English and its context; retranslated because the German named the flag `--root` where the English says "the root", and the three sibling keys for the same concept already say Wurzelverzeichnis.