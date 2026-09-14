---
kind: decision
state: resolved
ts: 2026-09-14T02:18:41Z
ordinal: 13
note: "resolveCardIn has no empty-result contract to extend: every arm of it answers a card or returns an error. An absence returned here would reach every caller as the card not existing, which is the same silent wrong answer in different clothing. Both sibling ambiguities in this codebase, dinah.ambiguous-workbench and dinah.ambiguous-column, are refusals for the same reason."
---
An ambiguous number is an error, not an empty result.