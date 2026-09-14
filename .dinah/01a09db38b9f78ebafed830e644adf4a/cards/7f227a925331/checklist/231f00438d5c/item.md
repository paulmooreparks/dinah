---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:42Z
ordinal: 1
note: "Verified 2026-09-12 on head 995636e, discharged by dinah-488 AC-6 and AC-7 per this item's own note: TestMigrateNumbersBuildsTheRegistry holds the tie-break over the handwritten old-format fixture (the earlier created card keeps 5, the later takes the next number above the high-water mark, the identical-timestamp pair breaks by ascending identifier), and TestMigrateNumbersIsDeterministicAndIdempotent holds the same total order across two independent copies. Both ran green in the -run match over internal/bench."
---
The migration builds the registry from existing card frontmatter and states its tie-break rule for two cards that already carry the same number: ties break by journal order.