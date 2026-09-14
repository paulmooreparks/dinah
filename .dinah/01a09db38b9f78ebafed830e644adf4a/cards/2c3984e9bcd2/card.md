---
title: A language tag Dinah does not ship is ignored rather than refused
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
Dinah accepts `--lang zz`, renders everything in English, and exits 0. A person who mistypes a tag gets output that looks correct and carries no sign that their language setting did nothing, and a script that sets the wrong tag reports success. Every other closed vocabulary this tool takes is checked and refused by name, so the language flag is the one place a wrong value passes silently. This card asks what `--lang` should do with a tag no catalog answers to.
