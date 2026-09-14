---
kind: acceptance_criterion
state: pending
column: c9428b3bc921
ts: 2026-09-14T02:16:58Z
ordinal: 2
---
Against a throwaway fixture repo (never the real checkout) with cwd classified "main", the guard allows "git rev-parse --path-format=absolute --show-toplevel" and "git worktree list --porcelain" by exact-string match after trimming, and denies "git rev-parse --path-format=absolute --show-toplevel && rm -rf x", proving the match is exact rather than prefix.