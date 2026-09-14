---
kind: acceptance_criterion
state: pending
column: c9428b3bc921
ts: 2026-09-14T02:16:58Z
ordinal: 7
---
scripts/hooks/record-isolation-cwd.py exists, ported from the Andoneer repository's file at the same path with worktree_kind/in_worktree adapted as specified (true for "linked", false for "main", null for "unknown"), and is registered in .claude/settings.json as its own PreToolUse entry distinct from both guards. Verified by test-record-isolation-cwd.py running the hook against three synthetic payloads (one per worktree_kind outcome) and asserting the appended isolation-audit.jsonl line's cwd_in_worktree field reads true, false and null respectively.