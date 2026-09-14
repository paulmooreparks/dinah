---
title: An empty list of checks reads as nothing failing
column: 2f6c18c9f5d0
state: ready
severity: major
priority: now
tier: workhorse
---
Two columns now require an agent to read a pull request's checks before handing a card on. Both tell it to wait for a check that is running. Neither says anything about the answer it will most often get first, which is that no checks exist at all.

Observed on dinah-293: after a push, the checks took roughly seventeen minutes to register against the new head, and the whole time the query answered that no checks were reported on the branch. An agent that polls once, reads an empty list, and finds no rule covering an empty list has broken no rule. It hands the card on having verified nothing.

It fails in the worst direction. A queued check at least looks unfinished. An empty answer looks like nothing-failing, which is exactly what a green result looks like, so the failure is invisible to the agent and to whoever reads its handoff.

The fix is wording in both the Implement and Agent Code Review columns, and it should say three things plainly: an empty answer is not a green answer; treat it exactly as a queued one and keep waiting; a stage that read the checks once and handed off has not read them. Whatever lands should also give the agent a bound, because "keep polling" with no ceiling is its own failure, and say what to do when the ceiling is reached rather than leaving it to invent a policy.

Worth deciding at the same time: whether anything can verify the rule rather than merely stating it. The rule these columns already carry was added because red checks reached the operator twice with nobody having looked, and this card exists because the wording that fixed it had a hole in the first case an agent actually met. A rule nothing checks is a rule that gets tested by the next incident.

Raised by Agent Code Review on dinah-293, which had no card-creation call available to it. Its proposed wording is in that card's round-three findings comment.
