---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:15Z
ordinal: 19
note: "The section 3.3 rule was argued on two grounds, that a set-wide write cannot be undone and that Dinah has no restore. This card removes the second, and the rule stands on the first: `dinah archive pb-1/comments` would be one act producing an unknown number of writes, and undoing it would take one restore per member with nothing recording how many there were. So the reason becomes that a reader cannot see what an act over a set did, and a per-member inverse does not give them that. restore joins the eleven refusing commands. The sidebar ruling belongs to a surface this card does not touch and was argued on the reversibility of a click, so it is left standing with a note in the spec that its premise has changed; reopening it means settling what that surface's confirmation looks like, which is work nobody has scoped. dinah-456 section 5.4's Restore column was already written forward and needs no edit."
---
dinah-456 section 3.3's refusal of a collection reference by every writing command survives this card on a re-stated reason, and the sidebar Archive ruling is not reopened.