---
title: The spec promises a field order the mechanism cannot give
column: 5ea2db0272fc
state: ready
severity: trivial
priority: soon
---
dinah-193's spec says severity and priority read in that order under `substate` "whichever of the two is present". That holds when both are written at once, which is what `dinah add` does. It does not hold when one is added to a card that already carries the other, because `SetAfter` inserts directly after its anchor and leaves an existing key where it sits, so the second key written lands above the first.

D-6 chose `SetAfter` deliberately and its reasoning stands: it keeps a key a person placed by hand where they put it, and it stops the pair landing below the workstreams block. The implementation is faithful to that decision and the code is right. The sentence in the spec is the part that overpromises, and both the implementer and the code reviewer flagged it independently.

The effect is cosmetic, since nothing reads the order and both keys are found by name. What needs fixing is the prose: the spec's sentence, and the same claim wherever it reached `docs/specs/dinah-193-severity-and-priority-ux-sketch.md`, which merged to `main` in commit 7db5be0. Either soften the claim to the simultaneous-write case it is true of, or say plainly that incremental placement follows whichever key was written second.
