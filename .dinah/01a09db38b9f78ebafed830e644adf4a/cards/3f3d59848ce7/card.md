---
title: The unknown-key refusal lists config settings whatever you were setting
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
Dinah's `unknown-key` refusal is hard-wired to list the config settings. Running `dinah workbench set <bad-field>` prints "lang, actor, editor, workbench", which are the settings `dinah config set` accepts and have nothing to do with a workbench's own fields. Somebody who mistypes a workbench field is handed a list that cannot contain what they meant, so the refusal actively points away from the fix.

This was found during the design review of dinah-193, which had planned to reuse the same refusal for a mistyped card field and would have inherited the same wrong list. dinah-193 now has to design its own answer for the card case; this card covers the defect that already ships, on the workbench case, independent of that work.

The fix is to make the listed set follow the entity being written rather than being fixed at the config settings.
