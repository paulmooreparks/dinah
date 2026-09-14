---
title: A control character in stored text breaks every row it lands in
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
A workbench title, a state title, a card title, and a block reason are all free text a person types, and nothing stops one of them carrying a control character. A tab, a carriage return, or an escape byte pasted in from another tool travels into the store and back out into a listing, where it moves the cursor without occupying a column and pulls every field after it out of place. The same byte reaches an agent through the machine form.

The row renderer specified on dinah-101 measures a control character as zero columns, which is the only honest answer for text it cannot see, so this is not a rendering defect and cannot be fixed there. The question is what Dinah does about the text itself: refuse it at the point a person supplies it, report it as a finding when check walks the workbench, or repair it.
