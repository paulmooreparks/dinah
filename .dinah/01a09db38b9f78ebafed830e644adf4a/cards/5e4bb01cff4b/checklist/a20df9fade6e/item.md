---
kind: decision
state: resolved
ts: 2026-09-14T02:17:01Z
ordinal: 16
note: "Word-boundary matching across all prose produces false positives: cmd.archive.summary's English is \"Move a card, a state, or anything below a card, out of the live set\", an ordinary verb, correctly translated into German as \"Eine Karte ... nehmen\" rather than left as \"move\". Backticks and token.* entries are where the catalog already marks a word as the literal thing itself rather than a description of it."
---
Contract-token matching is scoped to backtick-quoted spans and whole-text `token.*` entries, not every occurrence of a token word in English prose.