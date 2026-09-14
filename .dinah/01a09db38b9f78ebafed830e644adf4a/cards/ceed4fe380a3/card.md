---
title: Two guides open the same paragraph in two registers
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The Prose standard's documentation section puts `you` or `Dinah` in the subject slot of almost every sentence a reader learns from, and calls an abstraction standing there the loudest problem in documentation prose. `verbs.md` opens its claim paragraph with "`claim` takes up a card that is waiting. Work here is taken rather than handed out", where `Work` holds the subject slot. dinah-164 was told at design review to rewrite that same sentence in its own guide and did, so `first-session.md` now reads "You take work here rather than waiting to be given it" while `verbs.md` keeps the older form. Two guides a reader meets in the same sitting now differ in register on one idea. Both reviews on dinah-164 declined to fix the trunk sentence inside a diff nobody was reviewing for it, which is right, and it leaves the pair out of step until somebody moves them together.
