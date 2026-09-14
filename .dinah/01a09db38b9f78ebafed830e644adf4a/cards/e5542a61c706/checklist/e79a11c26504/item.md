---
kind: decision
state: resolved
owner: holder
ts: 2026-09-14T02:16:58Z
ordinal: 12
note: A command needing more than one hop to reach its worktree is refused. Trying to verify a cd took effect later in an arbitrary command is the unbounded shell-grammar problem deny-destructive-git.py's header already tried and rejected three times for git verbs; refusing more is the direction that guard is already permitted to be wrong in, and this one inherits the same posture.
---
The guard's cd-prefix and -C forms read only the command's first characters and chase no second cd or -C occurring later in the text.