---
kind: decision
state: resolved
column: c9428b3bc921
owner: operator
ts: 2026-09-14T02:17:03Z
ordinal: 39
note: "Ruled by the operator on 2026-08-27: strike the second selector. His words: it is more important that the translations make sense to the reader.\n\nSo a contract token quoted in backticks inside a sentence must survive translation byte-identical, and that is what shipped and is armed. An entry in the token.* family whose whole text is that token is not held to byte-identity, because carrying the human rendering of a canonical value is exactly what those entries are for, while the machine surface keeps the canonical spelling elsewhere.\n\nAC-4 has been amended to state only the surviving selector and is now verified. No code and no catalog entry changed."
---
The spec's second contract-token selector, "a token.* entry whose whole text is one token must survive byte-identical", is falsified by the catalog and is not implemented. Ratify dropping it, or redirect.