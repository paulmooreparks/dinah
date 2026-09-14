---
kind: decision
state: resolved
ts: 2026-09-14T02:17:01Z
ordinal: 18
note: "Grepped every one of en.json's 11 \"root\" occurrences by hand: four mean the root directory (\"the root\" present in all four), two are literal --depth vocabulary values (\"root, cards, entities, or all\") correctly left untranslated in both catalogs today, and five are flag references or {root} placeholders. A bare-word trigger would fail the two correct depth-vocabulary entries. Without placeholder-stripping, card.line's \"{ref} {title} [{state} / {substate}]\" would trigger the \"state\" term on its own placeholder despite carrying no prose. Without the context exclusion, param.tree.group-by.summary's declared-untranslatable \"state,substate\" would trigger it too."
---
The glossary's English trigger for "root" is the phrase "the root", not the bare word "root", and every glossary trigger strips `{...}` placeholders and skips entries whose context declares "never translated" before matching.