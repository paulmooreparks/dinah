---
title: The workbench journal is written and never read back
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
A workbench keeps its own journal beside its anchor, and the tool already appends events to it: an attachment against the workbench lands there, and so does the record of a card that was deleted. Nothing in the command surface reads it back. `dinah log` resolves its argument as a card, so `dinah log workbench` refuses the way every other workbench reference used to, and a person who wants to see what happened to the workbench itself opens the file. `dinah show workbench` refuses for the same reason, through a different resolver.
