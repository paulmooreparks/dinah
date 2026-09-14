---
title: The guide command answers in the machine form it is asked for
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
Every command that prints for a person also answers in a machine-readable form when asked, except the one that prints the guides, which ignores the request and prints for a person anyway. That was harmless while its output was a block of prose, and it stopped being harmless when tables gained headings, because a program reading it now receives a heading row it did not ask for and cannot distinguish from content. The behaviour predates the table work and was found by it.
