---
title: German and Hindi each translate "operator" two different ways in the help tables
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
In the printed help tables, German and Hindi each render the word "operator" two different ways. A reader of either language meets the same role under two names and has no way to know they are the same thing.

Found during dinah-409's code review by reading the rendered pages rather than the catalogs, which is the only way this surfaces: each entry is individually a defensible translation, and nothing compares two entries against each other for a term they share.

What this card has to settle beyond the two entries. Whether the project keeps a glossary of terms that must render identically across every message, since the fix for two entries is an edit and the fix for the class is a place where the decision lives. dinah-413's translation work already ran into the neighbouring problem from the other side, where neither language had a settled word for one of Dinah's own concepts, and recorded that in a translation record rather than anywhere a later translator would look for it.

Note the constraint the board already carries: no fluent reader of either language works on this project, so a card of this kind cannot verify that a chosen rendering reads naturally. It can establish that one term renders as one word, which is a claim about consistency rather than about fluency, and that is the honest scope for it.
