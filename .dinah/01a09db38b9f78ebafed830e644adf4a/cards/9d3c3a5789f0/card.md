---
title: A column setting written to disk does not reach a running machine-head session until it restarts
column: 5ea2db0272fc
state: ready
severity: major
priority: next
---
Found on 2026-09-10 during the code review of dinah-477, and it predates that card rather than being caused by it.

A column's declared properties are read into a running process, and a change written to disk does not reach that process until it starts again. Over the command line this is invisible, because each command is a fresh process that reads what is on disk. Over the machine head, which is a long-lived server, a session that has already read a workbench keeps what it read.

**Why this is worth a card now rather than whenever.** dinah-477 has just made a column's hold settable from the command line, which was the whole point of that card: turning the hold on should not require a text editor. But a session working through the machine head, which is how the agents on this project actually reach a workbench, will not see the hold until the process restarts. So the mechanism can be switched on and appear to do nothing, and the difference between "the hold is off" and "the hold is on and this session has not noticed" is invisible from inside that session.

That is the same shape this project spent two cards closing from other directions. dinah-473 landed because a hold could silently fail to fire when its column was named wrongly. This is a hold silently failing to fire because the process is looking at an older copy of the board. Both leave a workbench that reads as configured and enforces nothing, which is worse than one that plainly does not have the feature.

**What is not yet established, and should be before anything is designed.** Whether this is confined to the hold or applies to every declared property of a column, which the reviewer believed but did not prove. Whether it applies to other things a workbench declares beyond columns. How long a session in practice holds a stale copy, which decides whether this is a nuisance or a correctness problem. And whether anything already exists to reload, since a mechanism may be there and simply not wired to this path.

**What this card has to settle.** Whether a running session should notice a change to the board underneath it, and if so how it learns: re-reading when it next answers, watching the files, being told, or something else. Each of those has a cost, and the cheapest is not obviously right for a tool whose whole design is files on disk with no server, because a session that re-reads everything on every call pays for a guarantee most calls do not need.

Weigh also whether the honest answer is to leave the behaviour alone and make it visible instead, so that a person who turns a hold on is told plainly that sessions already running will not see it until they restart. That is a smaller change and it converts a silent wrong answer into a known one, which is the distinction that has mattered on every card of this shape so far.

**A constraint that applies to whatever is built.** The operator ruled on 2026-09-09 that Dinah must stay usable for workbenches with no code, no merge and no tests, so any wording this produces has to make sense to somebody running a renovation who has just turned something on and is wondering why nothing happened.
