---
title: Two ways past the destructive-git guard that survive on the trunk
column: 5ea2db0272fc
state: ready
severity: major
priority: next
---
The guard that keeps destructive git commands out of the operator's checkout can still be walked past in two ways, both found by attacking it with a real shell and a recording stub. Neither is a spelling any agent on this board writes, and both are present in the guard already deployed as well as in the branch that dinah-233 is landing, so neither is a regression.

The first is a separator nested inside a command substitution or a brace expansion. The guard's reading of a command ends early at that separator and the verb after it is never seen, so a bare reset runs. This is the same shape dinah-233 closed at the outer level, one level further in.

The second is the more serious of the two and it is the one that lands where the guard exists to protect. The reader that extracts an explicit worktree path stops at a closing quotation mark, where the shell reads straight through it, so a path that begins as a worktree and then climbs back out is accepted and git is handed the operator's own checkout. The unquoted spelling of the same path is correctly refused.

Both were left out of dinah-233 deliberately. That card had run seven implement-and-review cycles, its cost had been called unacceptable, and refusing it over defects its own replacement also has would have discarded the fixes without closing these. The reviewer that found them recommended a separate card, and this is it.
