---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:15Z
ordinal: 23
note: "Round 1's review filed this as a nit, observing that all ten decisions gated the column the card was standing in and that a gate naming the current column cannot fire. The observation is right from where the review sat, because the card had already moved into Agent Design Review by the time it was read. The gate is still correct. The Spec column's own instructions rule that a decision the spec author takes here is settled at Spec and therefore gates the column after Spec, and the workbench instructions say the same thing in their own words: an open question or a decision gates the column after the one that answers it, and a gate refuses entry, so gating the answering station itself would lock the card out of the place its answer comes from. Agent Design Review is the column after Spec on this card's lane. The alternative readings were both rejected: moving the gates to Build Queue is the rule for an operator-owned question rather than for a decision settled here, and moving them to Agent Code Review is the rule for a decision the implementer takes in the course of the work, which the Spec instructions name as the specific mistake four decisions on another card made on 2026-09-08. All eleven decisions on this card are resolved, so every one of these gates is inert either way; this item exists so the next card copies the prescribed pattern rather than learning the wrong lesson from the nit."
---
Every decision on this card gates Agent Design Review, and that is the prescribed gate rather than a copied mistake, so the gates are left where they stand.