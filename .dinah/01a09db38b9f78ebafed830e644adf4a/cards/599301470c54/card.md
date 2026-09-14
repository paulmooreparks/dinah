---
title: Severity and priority level sets are declared and read
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
A card may carry a `priority:` or `severity:` key in its frontmatter and Dinah preserves it untouched, but nothing reads either one and no workbench says how their values rank. `docs/design/format.md` designs a `levels:` block in the workbench definition that supplies the ranking, low to high, with an optional one-line hint per level, and no code implements it. That leaves a whole class of question unanswerable: what is ready above a given priority, which is one of the three questions dinah-135 was filed on, and which its query language refuses today because the field does not exist. The work is to declare the level sets, read them, and let the rest of the tool rank on them.
