---
title: The workbench listing comes back in the same order twice running
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
Listing the workbenches you can reach sorts the rows by the twelve-character identifier each one was minted with, which nobody sees and which is random, so two runs of the same command can put the rows in different orders. Creating the same pair of workbenches ten times gave one order six times and the other four. The same listing appears in two more places, so the inconsistency is not confined to one command. The operator ruled the general form of the key rather than today's: order by a priority where one exists and the table shows it, then by a severity on the same condition, then by title, then by slug. A workbench carries neither of the first two today, so that reduces to title with slug as the tie-break, and it will not want rewriting the day either field arrives.
