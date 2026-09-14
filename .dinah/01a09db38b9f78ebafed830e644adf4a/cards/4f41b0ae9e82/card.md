---
title: A new workbench declares no severity or priority levels
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
`dinah init` writes no `levels:` block, so a brand-new workbench declares no severity and no priority levels at all. Once dinah-193 lands the write path, every `dinah add --severity` and every `dinah card set <card> severity` on such a workbench refuses with `dinah.no-levels` until a person opens `workbench.md` and writes the declaration by hand. The refusal is correct and it names the file, so the way out is discoverable, but it is a poor first experience of a feature that should work out of the box.

The operator decided on 2026-08-23 that `init` should seed both sets, using the members `docs/design/format.md` already carries as its own example, which is also what the Dinah development board declares: `severity: [trivial, minor, major, critical]` and `priority: [later, soon, next, now]`. A workbench that wants different names edits four words; one that wants none deletes two lines. This mirrors `states`, which `init` already seeds.

Kept out of dinah-193 deliberately: seeding changes what `init` writes, which changes the stored compatibility fixture, and dinah-193 already covers the level model plus the card write path. See D-8 on dinah-193 for the decision as recorded.
