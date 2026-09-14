---
title: The extension's command titles have no consistent naming pattern
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The operator's screenshot on 2026-08-30 found that of the extension's declared commands, most appear bare in the Command Palette (Claim, Release, and so on) rather than carrying the product name, while a couple do. dinah-342 hides every row-scoped command from the palette, which stops this mattering for commands that are row-scoped, but nothing stops a future command from being both palette-visible and inconsistently titled.

Flagged during dinah-330's spec (OQ-2) as out of scope for that card, since dinah-330's own two new commands are row commands and inherit dinah-342's palette hiding. The underlying question, a naming convention for palette-visible commands across the whole extension, is unresolved and untouched by any existing card.
