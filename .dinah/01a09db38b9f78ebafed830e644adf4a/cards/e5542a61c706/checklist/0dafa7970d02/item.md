---
kind: acceptance_criterion
state: pending
column: c9428b3bc921
ts: 2026-09-14T02:16:58Z
ordinal: 9
---
Not automatable by this repository's build or test pipeline: the operator or an agent with board-admin access applies the isolation directive rename to "scratch-worktree" on the Spec, Implement, Agent Code Review, Test and Merge columns, and sets a new workbench-scope scratch_root directive of C:\\dinah-scratch, directly against the live Andoneer board. Verified by get_column(fields="effective_directives") on each of the five columns after the change.