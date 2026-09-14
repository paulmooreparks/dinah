---
kind: acceptance_criterion
state: pending
column: c9428b3bc921
ts: 2026-09-14T02:16:58Z
ordinal: 4
---
Against the same fixture, the guard allows "cd <linked worktree> && go build ./..." (quoted and unquoted path), and denies the main-checkout form, a form where cd is not the command's first token ("echo hi; cd <worktree> && ..."), and a form where the cd target is not a worktree at all.