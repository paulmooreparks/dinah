---
title: Refusals carry the file path only when raised from one place
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
A shared step attaches the workbench file's path and a fix instruction to a refusal, but only to refusals raised while the workbench is being opened. A refusal raised anywhere else must carry that guidance in its own wording or go without.

The result is already visible. Two refusals added by one card for the same underlying condition differ in what they tell a person: one names the file to edit, the other does not, so whether you learn where to go depends on which command you happened to run.

The card decides whether the shared step should apply wherever a refusal concerns a particular workbench, and what that means for the messages currently carrying their own copy of the guidance.
