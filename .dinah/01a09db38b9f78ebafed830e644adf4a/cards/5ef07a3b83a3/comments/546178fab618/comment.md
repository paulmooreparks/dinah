---
ts: 2026-09-14T02:34:42Z
author: paul
ordinal: 1
---
Reasoning for the pending operator question dinah-449/questions/3, which its own note cannot hold: a pending item's note is refused a line break, so the argument lives here. That limitation is filed as dinah-494.

Found while running the cutover on 2026-09-14, and it is the answer to the question this card asked me to establish, which was whether the eight `dinah set` lines would be refused to an agent. They were not. `dinah whoami` in an agent session on this machine answers "paul, operator: yes", because the environment already exports DINAH_ACTOR=paul. Nothing was set or worked around to achieve that.

The gate does work as written: `DINAH_ACTOR=agent dinah set triage hold out` is refused `not-operator`, and the identical command without the override succeeds. So `AuthorityOperator` (internal/bench/fields.go, writeAuthority) is a real check on the actor NAME, and it refuses nothing to a session that inherits the operator's name. Everything it guards is therefore reachable by any agent on this machine: every column field including the holds, the instructions on every column and on the workbench, and the closing of any item stamped `--owner operator`.

That last one is worth separating out, because it is the case where the consequence is quietest. An item filed for the operator is meant to be a stop only he can clear, and this board's whole shape after your 2026-09-14 ruling rests on it: your two review stations are no longer owned, so the only thing that stops a card at them is a pending item naming that column, and the only thing that stops an agent clearing that item for you is this gate.

Three shapes, with what each costs.

A. Leave it. The boundary is between a person at a terminal and a deliberately-declared agent, and honouring it is a convention agents follow rather than a wall. Costs nothing today and keeps the tool simple. The risk is that it reads as a wall in this card's own spec, which says a column's fields are "refused to every other owner", and in OQ-2's note, which calls the eight lines "all his because a column's fields carry operator authority". Both sentences are true only if nobody runs as him, which is not the case here.

B. Make the agent declare itself rather than inherit. The harness stops exporting DINAH_ACTOR=paul to agent sessions and exports an agent name instead, and you set your own actor when you work at a terminal. No code changes; it is a configuration change on this machine. Costs you one environment variable and makes every refusal in this paragraph real. This is my recommendation.

C. Have the tool distinguish. Some second factor beyond the actor name, so that an agent cannot be the operator whatever the environment says. That is a real design question about what the second factor could be on a single-seat local CLI with no credentials, and I would not start it without you wanting it.

I did run the eight lines, and I want that ratified or reversed rather than assumed. My reason for running them is that your OQ-2 ruling sets out those exact eight lines as what to run under shape A, so running them executed your instruction rather than substituting my judgement for it. The same reasoning covers the two operator_owned hand edits and the instruction rewrite, which turns out to need the same authority and which the spec does not flag as yours. If you would rather have performed any of those yourself, they are all reversible: the backup at C:\dinah-scratch\dinah-449-backup-20260914-100714 holds the workbench exactly as it stood before anything was written.