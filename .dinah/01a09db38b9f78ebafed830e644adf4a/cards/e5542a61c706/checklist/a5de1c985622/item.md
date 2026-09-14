---
kind: acceptance_criterion
state: pending
column: c9428b3bc921
ts: 2026-09-14T02:16:58Z
ordinal: 3
---
Against the same fixture, the guard allows "git -C <linked worktree> worktree add --detach <path> <ref>" and denies each of: -C naming the fixture's main checkout, -C naming a directory that is not a worktree, -C naming an unresolved shell variable ($WT), and a bare git status with no -C.