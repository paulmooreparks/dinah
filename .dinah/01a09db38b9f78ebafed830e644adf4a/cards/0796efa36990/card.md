---
title: Filing an item with a column's short name produces a hold that never fires
column: b69abf918c42
state: ready
severity: major
priority: next
tier: workhorse
workstreams:
  - b3f924406e4c
links:
  - kind: relates_to
    to: 211cecf3660d
---
The operator ruled on 2026-09-10, answering a question on dinah-450, that the file command should resolve a column reference at write time the way the move and pull commands already resolve theirs, and refuse when the reference names nothing.

**What happens today.** The write path stores whatever column value it is given, verbatim, and nothing resolves it. The hold that dinah-450 introduces matches on the column's identifier. So filing an item against a column by its short name produces an item that names no column the workbench knows, the hold never fires, the card passes the step the workbench meant to hold it at, and nothing anywhere reports any of it.

**Why it matters now and did not before.** Until dinah-450 the field was decoration: the only reader that consumed it resolved it through a lookup that takes identifiers and nothing else, so a value that was not one produced a blank title in one view and cost nothing. The hold makes the field load-bearing, and the same typo now silently disables a control.

That shape is worth naming because this board keeps meeting it. Everything looks configured, nothing is enforced, and the only evidence is the absence of a refusal that should have happened. It is a refusal any value satisfies, arriving from the writing side rather than the reading side.

**What the fix is.** Resolve the reference through the same path the other column arguments use, and refuse an unknown column at write time rather than storing something that will never match. Existing items are unaffected: they already carry identifiers in practice, for the reason above.

**Why the operator chose this over the cheaper half.** A health-check finding was the alternative, and it would have reported afterwards that an item had been holding nothing. He took the write-time refusal because it means the item never exists in that state at all. A check that tells you later is worth having and is not a substitute for not creating the problem.

**What this card should settle beyond the obvious.** Whether the refusal names the unknown column back to the caller, since a person who typed a short name needs to see which of the two forms the tool wanted. Whether anything else on the write side stores a reference without resolving it, established by looking rather than assumed, because a defect of this shape in one place is worth checking for in its neighbours. And whether the health check should also grow the finding, since the two are not exclusive and existing items written before this lands would still carry unresolvable values if any exist.

## Branch

dinah-473-filing-an-item-with-a-columns-short-name-produces-a-hold-that-never-fires
