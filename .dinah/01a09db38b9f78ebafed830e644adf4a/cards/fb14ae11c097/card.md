---
title: The quick start teaches only one machine format
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The quick start's prose names `DINAH_FORMAT=json` and stops there. dinah-31 added a second machine form, reached by `--format compact` and `DINAH_FORMAT=compact`, and the quick start is where an agent learns what this tool offers.

Deferred out of dinah-31 on 2026-08-27 rather than folded into it. The spec scoped that card to the help block and the message catalogs, and there is a mechanical reason the deferral is right rather than lazy: the quick start is a continuously replayed session and three fixtures key on its line numbers, so editing its prose shifts lines that `cmd/dinah/testdata/uncovered.txt`, `cmd/dinah/testdata/quickstart-exempt.txt` and the `site:` fields in `sweptBlocks()` all record. Regeneration cannot see prose, so the realignment is done by content match rather than by arithmetic, which is the discipline dinah-133 established.

What this card wants is a decision about how much the quick start should say, not simply a second environment variable added to a sentence. The compact form exists for a driver loop rather than for a person reading a terminal, so the question is whether the quick start teaches it at all, mentions it in passing and points at the guide, or shows it working. Each answer costs a different amount of that fixture realignment.

Worth reading first: dinah-31's own decision about the format's version marker, and its open question OQ-2 on whether the format is a stable promise. If the operator later rules the format unstable, teaching it prominently in the quick start is a different proposition from teaching it as settled.
