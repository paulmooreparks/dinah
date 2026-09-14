---
kind: open_question
state: resolved
owner: holder
ts: 2026-09-14T02:17:03Z
ordinal: 41
note: "No inflected form of Akteur appears anywhere in de.json today (checked by hand, grepping Akteur[a-zA-Zäöüß]* across the file: only the bare form is present). The six-entry backfill this spec requires may introduce a grammatical context (a genitive or plural) the bare form does not cover. Same path as OQ-1: Implement runs TestATranslationUsesTheDeclaredWord against the real, backfilled catalog; if it fails on a form that is a legitimate German inflection rather than a wrong word, add that form to glossary.json rather than loosening the trigger or the match.\n\nPaul: As above, I'm not sure."
---
Is "Akteur" the complete set of German inflected forms the "owner" glossary term needs, or does an existing or backfilled entry legitimately need a further inflection (e.g. "Akteurs", "Akteure")?