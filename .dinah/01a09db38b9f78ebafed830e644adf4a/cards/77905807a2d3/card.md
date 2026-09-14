---
title: Refusing to archive the workbench root rests on one line no test exercises
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
Archiving the workbench itself, written as `dinah archive .`, is refused by a single line inside the archive command, and no test in the tree exercises that line. Delete it and the command would archive the workbench root and report success, with nothing in the suite going red.

This was found during the code review of dinah-180 and deliberately left out of that card. dinah-180 is about references the workbench cannot resolve, and this reference resolves perfectly well. It is refused because a workbench is not a thing you may archive, which makes it a cousin of the occupied-column refusal rather than a fourth unresolvable reference. Widening that card to cover it would have blurred what it was proving.

The gap matters more than its size suggests, because of what the unguarded line protects. Every other refusal in that command declines to touch one entity. This one declines to move the root that every other entity hangs from, so the failure it prevents is not a bad exit code but a workbench folded into its own archive. A reader deleting the line while tidying would see a green suite.

What is wanted is a test that runs the refusal and asserts both the refusal name and the exit code, placed where the exit-code table already covers the command's other refusals so the next reader finds it beside its siblings. Check while you are there whether any other refusal in that command is similarly unguarded, and establish that by reading the command's refusal paths rather than by searching for the message, because this board has repeatedly found that a survey by wording misses what a survey by meaning catches.
