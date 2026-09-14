---
title: Selection offers a card whose claim would be refused for an unanswered question
column: 5ea2db0272fc
state: ready
severity: major
priority: next
---
The command that hands an agent its next card does not filter on unresolved checklist items, so it will offer a card whose claim is then refused because the card carries an unanswered question or an unsettled decision. An agent asks for work, is given a card, reaches for it, and is told it cannot have it.

This is the same defect dinah-410 fixed for tiers, on a different row of the same gate. That card consolidated the tier comparison so the offer and the refusal could not disagree, and its whole justification was that an agent must never be shown a card and then refused it. The unresolved-item row of the claim gate was never given the same treatment, and it predates dinah-410 rather than arriving with it.

Found while sweeping for other instances of the shape after dinah-410's review caught one. The sweep was done by enumerating the gate's exemptions rather than hunting for similar-looking code, which is why it turned this up: the tier check has exactly two call sites and one exemption, so it was clean, and the neighbouring row was not.

Two things to settle rather than assume.

Whether the fix is to filter, or to answer differently. Filtering hides the card, which is right if an agent can do nothing about the unanswered question, and wrong if the card is the agent's own to resolve. Read who owns an unresolved item and whether the asking agent could settle it, because a card silently vanishing from a queue is its own kind of defect and this board has a rule against a queue that lies about what is in it.

Whether any other row of the claim gate has the same gap. dinah-410 established the method: enumerate what the gate checks and ask, for each row, whether selection consults it. Do that once for every row rather than for this one, so the answer is a statement about the gate rather than about two of its rows.

Note what dinah-410's implementer found about the tier row while fixing it, since the same trap sits here: an empty declaration is not a neutral input in this code, it is the strictest one, and overloading the empty value with a second meaning is what produced the original bug. Do not reach for the same shortcut.
