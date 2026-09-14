---
kind: acceptance_criterion
state: pending
column: c9428b3bc921
ts: 2026-09-14T02:16:58Z
ordinal: 1
---
scripts/hooks/deny-main-checkout-cwd.py exists, guards main() behind __name__=="__main__", and imports worktree_kind from deny-destructive-git.py rather than re-implementing worktree classification. Verified by test-deny-main-checkout-cwd.py asserting the imported worktree_kind is the same function object as deny-destructive-git.py's own.