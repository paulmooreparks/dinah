---
title: A directory dinah check cannot read is swept as empty, so a permission failure reports as no defects found
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
---
`dinah check` walks the workbench looking for defects. When a directory in that walk cannot be read, the sweep treats it as holding nothing and the check reports no finding for it. A person whose permissions are wrong, whose file is locked, or whose storage is failing gets the same answer as a person whose workbench is genuinely clean: confirmed clean.

This is the defect family this workbench has closed eleven times in recent weeks, and this is its largest remaining surface. Every earlier instance was a read failure arriving as an empty collection, and the emptiness then reading as a confident negative answer. The others were a status bar, a refusal reply, a container listing and a palette entry. This one is the tool whose entire job is to answer whether anything is wrong, which makes a false clean here worse than a false clean anywhere else.

It was found by code review on dinah-459 while checking that card's new sweep. It is not a defect that card introduced: the new sweep follows the house rule the whole check surface already uses, and the reviewer said so explicitly. That is the reason this is filed as its own card rather than as a finding there. The scope is the surface, not the one sweep.

What the card should settle, none of it the operator's to rule on:

How many sweeps the check surface runs and which of them swallow a read error. Establish the set by reading the code rather than by fixing the one that was reported, because the point of this card is that the rule is surface-wide.

What a reader is told when a sweep cannot read what it was asked to read. It has to be distinguishable from a clean answer and from a defect finding, since it is neither: the check did not find a problem with the workbench and it did not confirm the workbench is sound, it failed to look. The exit code needs the same treatment, because a caller scripting `dinah check` reads that rather than prose, and exit code two already means both a refusal and a workbench with findings, which dinah-346 dealt with once.

Whether the machine surface says the same thing. `check --json` details already arrive raw, and an agent reading a clean result and acting on it is the same failure with no human in the loop to smell it.

Whether the fix belongs at each sweep or at whatever they share. Eleven earlier instances were fixed one route at a time until the fixes started changing types instead, which is what finally held: a report became one of three states so a failure could not silently remove a place from the list, and an exclusion list became an inclusion set so an unknown case falls through to doubt rather than to confidence. Read those before choosing, because patching each sweep is how this family survived ten fixes.

Related: dinah-433 (a container listing that read an unlistable directory as an empty one), dinah-434, dinah-437, dinah-439 and dinah-440 are the open members of the same family. dinah-346 is the precedent on what exit code two already carries.
