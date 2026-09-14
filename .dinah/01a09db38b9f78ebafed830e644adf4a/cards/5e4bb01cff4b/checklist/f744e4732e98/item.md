---
kind: decision
state: resolved
ts: 2026-09-14T02:17:01Z
ordinal: 19
note: The contract-token guard selects structurally (backticks, token.* keys) against a fixed declared vocabulary and never reads dinah-258's context-phrase selector, so it is unaffected either way. The glossary guard's context exclusion does reuse the same "never translated" phrase dinah-258 says is incomplete; that risk is assessed separately in the spec's "dinah-258, and whether it undermines this card's guards" section, which found no live entry affected today and recommended (without requiring) that dinah-258's own fix update this guard's exclusion too if it becomes a shared structured signal. Corrected from the first draft, which scoped this claim to "this card's guards" as a whole rather than to the contract-token guard specifically.
---
dinah-258 stays its own card; the contract-token guard neither depends on nor duplicates its selector.