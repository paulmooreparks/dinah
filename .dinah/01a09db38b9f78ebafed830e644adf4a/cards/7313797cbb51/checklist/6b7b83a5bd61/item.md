---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:25Z
ordinal: 20
note: "Round 1 recorded this as a reading of internal/bench/resolve.go lines 404 to 421 that nobody had run, and Agent Design Review asked for the reproduction. Running it makes the finding worse rather than dissolving it. A column slug cannot do this, because `dinah set doing slug 1`, `slug 12` and `slug 1x` are each refused as malformed while `slug x1` is accepted, so ValidColumnSlug admits no slug opening with a digit. A column title is unconstrained: `dinah set done title 1 --yes` succeeds, and afterwards `dinah path 1` answers the column's column.md and `dinah show 1` prints the column, while `wb-1` goes on opening the card. A twelve-hex column slug is admitted too, which WorkstreamByRef's own doc comment already records, so the identifier form has the same exposure.\n\nNot narrowed, because changing which entity an existing reference names is a compatibility change and not this card's subject; the column-before-card ordering is the same deliberate shape as the card-before-workstream ordering orAWorkstreamNamedBarely records. Taught instead, in the same paragraph that teaches the form it affects, because a guide that teaches `dinah show 1` and says nothing about precedence teaches something silently false on any workbench with a numeric column title. AC-1 holds the sentence."
---
The guide teaches the precedence that lets a column shadow a bare card number or a bare card identifier, and the resolver is not narrowed.