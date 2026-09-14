---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:11Z
ordinal: 25
note: "`cmd/dinah/commands.go`'s own comment on the `card` entry says the groups split on what a command acts on, and that `groupWork` holds every command that acts on a card. These two act on any entity of the workbench, which is what `path` and `edit` already do from `groupBench`, and `edit` is the command they are replacing as the route to prose. Putting them in `groupWork` would say to a reader scanning `dinah help` that they are card commands, which is the one thing they are not."
---
`get` and `set` join `groupBench` rather than `groupWork`, beside `path` and `edit`.