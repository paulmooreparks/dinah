---
title: A German check line states a condition as fact where the English qualifies it
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
One of the German `check.*` lines drops the "if carried" qualifier the English carries and states the condition as fact. The English says a marker is checked if one is present; the German says it is checked.

Two passes over dinah-248 agreed this is a real defect, agreed it was outside that card's two named fixes, and neither filed it. It is filed here so it stops being the thing everybody notices and nobody owns.

It is the same class as the defect dinah-248 did fix, where the German MCP summary said "this workbench" after the server had gained a root and could serve many. The German is fluent, current in form, on-glossary, and says something the English does not. No mechanical check catches it, and that is precisely what makes the semantic layer on dinah-252 worth its cost.

**The fix.** Identify the entry, restore the conditional sense in German, and check the same line in Hindi while there, since the two catalogs have repeatedly turned out to share a defect when one is found. Leave the five skeleton catalogs alone; they carry the English verbatim and are correct by construction.

**Worth an extra ten minutes.** The `check.*` family is a list of preconditions, and several of its English entries qualify a condition rather than asserting it. Read the family in both translated catalogs against the English and say whether this is the only line where a qualifier was dropped. One instance is a slip; three would be a pattern in how these were translated, and that changes what dinah-252's semantic layer has to look for.
