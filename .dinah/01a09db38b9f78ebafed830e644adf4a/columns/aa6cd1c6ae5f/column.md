---
title: Done
slug: done
kind: done
operator_owned: false
---
The last column. A card here has completed its work, its commit is on the trunk, and it was accepted there without incident.

### Discipline

- **No work happens here.** Cards in Done are read-only by convention, and the card's journal is the audit trail.
- **No reopening.** When a Done card needs revisiting, because of a regression, a follow-on, or work that grew out of it, file a new card and link it with `dinah link <new> <the done card> relates_to`. Do not drag the Done card back into the flow.
- **Comments after Done are allowed but rare.** A cross-reference or a link to follow-on work is fine; substantive new content belongs on the new card.

Read the second of those as a convention rather than as a refusal. Nothing in the tool stops a card being moved out of this column, by any actor, so what keeps Done terminal is this instruction and nothing else.

### What being Done means

The card's contract is satisfied, its acceptance criteria are verified, its commit is on the trunk, and the change has been exercised without incident. Merge landed it and Acceptance watched it.

Done therefore means as far as this workbench currently ships. A card is Done when it has passed every gate the workbench has, whatever those are at the time.

### This column holds nothing

Nothing is settled here and nothing has to be settled before a card arrives, so this column declares no hold and no reject target.
