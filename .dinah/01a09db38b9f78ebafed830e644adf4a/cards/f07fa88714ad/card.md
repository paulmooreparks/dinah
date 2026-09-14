---
title: A command accepts a flag it never reads
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
Dinah's argument parser checks a flag against one list of every flag any command declares, so `dinah ls --override` parses, runs, ignores the flag, and exits 0. A person who reaches for a flag on the wrong command is told nothing, and a script that carries a stale flag through a command change keeps reporting success while doing something other than what its author wrote. The parser already refuses a flag no command declares, so the machinery for refusing this one is in place and only the per-command scope is missing. This card asks whether a flag the named command does not read should be refused.
