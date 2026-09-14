---
title: A card's merge is checked while it waits, so waiting makes it stale
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
Three cards in two days have been sent back, or blocked at a loop ceiling, for the same reason: the branch stopped merging while the card sat in a queue. Implement is told to keep the trunk merged into the branch, Agent Code Review treats a branch that no longer merges as a defect of the diff, and Test merges the trunk again because the merged state is what it is asked about. So a card's merge is verified at several points and then goes stale in the gap before the next one, and the gap is exactly where a card waits for a reviewer or for the operator.

The cost is not the merge, which is cheap. The cost is that staleness enters the review loop as though it were wrongness, so the card takes another lap, and the lap is another interval in which the trunk can move again. dinah-158 needed an operator ruling to escape that loop after four push-backs, two of them for this same cause. dinah-151 reached its ceiling and hit the identical conflict in the identical two files. Each lap also re-runs a review of work nobody has changed.

What makes this board especially exposed is that three files are touched by nearly every card: the eight message catalogs, the replayed quick start, and the table-site registry in the row sweep. Two of them carry derived values, a catalog key count and a source line number, so any card editing them invalidates every other card in flight, and the conflict resolves to a value neither branch holds.

The change to weigh is moving the trunk merge to the end, where nothing can move under it afterwards. The Merge column already builds the merged result before landing it and already pushes back when that build fails, so the machinery exists; the question is whether the stations before it should stop asking, and what Test is then verifying if it tests a tree that is not the one that lands. This card carries a change to the board's own column instructions rather than to the tool, so it needs the operator's ruling on the route as well as a spec.
