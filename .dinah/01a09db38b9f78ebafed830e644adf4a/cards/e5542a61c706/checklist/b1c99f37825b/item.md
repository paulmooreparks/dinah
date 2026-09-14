---
kind: decision
state: resolved
owner: holder
ts: 2026-09-14T02:16:58Z
ordinal: 11
note: This guard has an opinion about the one repository it ships inside. A cwd naming no worktree of that repository at all is outside what this card is claiming to protect, and refusing every command whenever git cannot classify a directory would turn a repository-scoped guard into a machine-wide one.
---
worktree_kind(cwd) == "unknown" fails open (the guard allows the call).