---
title: One card's guard verification ran against an oracle that was answering nothing
column: 5ea2db0272fc
state: ready
severity: major
priority: next
---
The differential harness at `scripts/hooks/test-guard-against-a-real-shell.py` compares the guard under test against two oracles: a real shell, and the guard as deployed on the trunk. The second oracle was dead.

Its worktree-kind cache wrapper took two arguments where the function takes one, so every trunk verdict on a command carrying `-C` raised a `TypeError`. The exception was swallowed and the result recorded as "the deployed guard allows this". A `-C` is precisely what this guard exists to read, so for the commands the oracle was written for, it was answering nothing at all, and answering it in the direction that hides a regression.

Found and fixed during dinah-293, which also made an unanswerable trunk verdict fail the run rather than pass quietly.

The blast radius is small and known. Agent Code Review traced it: the harness has exactly one earlier commit and nothing has touched these scripts since, so one earlier card ran its regression check blind for every `-C` command. That card is dinah-233. Its verification is not known to be wrong, only unverified in the half that mattered most.

The work here is to retire that gap rather than assume it: re-run the now-live harness against dinah-233's change with `TRUNK_REF` pinned to `89d8701`, and record what it reports. If it comes back clean, the gap is closed and the card is done. If it does not, whatever it finds is a real regression that has been sitting on the trunk since.

Worth noting for whoever picks this up, because it is the same lesson twice: after the oracle was repaired, the harness still ran 37,721 strings and reported all clear while the entire deny set was open, because its generator never placed a verb inside a shell expansion. A test apparatus can be alive and still be looking in the wrong place. Confirm the generator reaches the class you care about before you trust a clean run.

Recorded on dinah-293 as OQ-1 by the implementer, which had no card-creation call available to it.
