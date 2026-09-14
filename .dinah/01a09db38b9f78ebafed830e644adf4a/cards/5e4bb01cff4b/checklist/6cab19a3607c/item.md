---
kind: decision
state: resolved
ts: 2026-09-14T02:17:02Z
ordinal: 26
note: Round two of design review found a live divergence (check.card.4 against its near-twin check.add.5) the duplicate-English-grouping method structurally cannot see, since the two keys' English text differs. A full grep of en.json for the bare word "level" (9 occurrences) confirms all nine mean the same concept; German drops the word on check.card.4 only, Hindi is already correct on all nine.
---
A fourth glossary term, `level`, is seeded alongside `state`, `the root` and `owner`.