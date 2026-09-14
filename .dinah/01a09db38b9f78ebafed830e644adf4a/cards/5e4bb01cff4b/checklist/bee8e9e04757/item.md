---
kind: decision
state: resolved
ts: 2026-09-14T02:17:02Z
ordinal: 27
note: "No go test can fail for a term nobody declared; that is a permanent limit of a seeded declaration. Design review asked that the term-adding mechanism be specified rather than named, since it is the real deliverable given the seed set is provably incomplete. The answer is the same enforcement surface Layer 3 already uses (Agent Code Review's column instructions): a diff changing en.json's key set or an existing key's English text without a corresponding glossary-sweep check is a [major] finding."
---
The mechanism for adding a glossary term later is a review-time check wired into Agent Code Review's own column instructions, not a claim that any test here can catch an undeclared term.