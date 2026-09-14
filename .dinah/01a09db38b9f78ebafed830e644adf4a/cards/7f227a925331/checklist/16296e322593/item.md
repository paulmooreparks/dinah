---
kind: decision
state: resolved
owner: holder
ts: 2026-09-14T02:18:45Z
ordinal: 35
note: "Round 1 rewrote the English text and context of this key and rewrote the German and Hindi texts in the same diff, so both translations were read against the new English rather than left matching the old one, and both carry source e0a7083f7826dbbf, the fingerprint of the current English text, verified by computing it. Per the ruling of 2026-08-31, mechanically edited translations ship without a fluent reader: an agent edited both texts against the new English and its context, no fluent reader has seen either, and this record claims no verification of their fluency. The eighteen keys the diff added are additions rather than changes, so the column's rule does not reach them."
---
`check.card-number-duplicate`: read against the current English and its context; retranslated because the English moved from '{detail} is a card number two cards carry, so a reference by number answers with either of them' to '{detail} claims a number that another line claims too, so a reference by number answers with more than one card', and the context grew the repair sentence naming `dinah check --renumber --yes`, so a translation matching the old wording would misdescribe both the finding and its repair.